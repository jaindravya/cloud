package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"cloud/internal/executor"
)

// worker registers with the api and runs jobs via the c++ executor
type Worker struct {
	apiURL     string
	workerID   string
	exec       *executor.Runner
	server     *http.Server
	mu         sync.Mutex
	registered bool
}

// new creates a new worker
func New(apiURL, workerID string, exec *executor.Runner) *Worker {
	if workerID == "" {
		workerID = "worker-" + randomID()
	}
	w := &Worker{apiURL: apiURL, workerID: workerID, exec: exec}
	mux := http.NewServeMux()
	mux.HandleFunc("/run", w.handleRun)
	mux.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) { rw.WriteHeader(http.StatusOK) })
	w.server = &http.Server{Handler: mux}
	return w
}

func randomID() string {
	const chars = "abcdef0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[i%len(chars)]
	}
	return string(b)
}

// run registers with the api and starts the http server (blocking). use start + shutdown for graceful shutdown.
func (w *Worker) Run() error {
	if err := w.Start(); err != nil {
		return err
	}
	select {}
}

// start registers with the api and starts the http server in a goroutine
func (w *Worker) Start() error {
	if err := w.register(); err != nil {
		return err
	}
	go w.heartbeatLoop()
	w.server.Addr = ":9090"
	go func() {
		log.Println("worker listening on :9090")
		_ = w.server.ListenAndServe()
	}()
	return nil
}

// shutdown gracefully stops the worker server (drains in-flight requests)
func (w *Worker) Shutdown(ctx context.Context) error {
	return w.server.Shutdown(ctx)
}

// register posts to api /workers with this worker's id and endpoint.
// we need our own url, in docker the api will reach us by hostname. for local dev we use a default.
func (w *Worker) register() error {
	selfEndpoint := getEnv("WORKER_ENDPOINT", "http://localhost:9090")
	body := map[string]string{"id": w.workerID, "endpoint": selfEndpoint}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	resp, err := http.Post(w.apiURL+"/workers", "application/json", &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register: API returned %d", resp.StatusCode)
	}
	w.mu.Lock()
	w.registered = true
	w.mu.Unlock()
	log.Println("Registered with API as", w.workerID, "at", selfEndpoint)
	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func (w *Worker) heartbeatLoop() {
	tick := time.NewTicker(20 * time.Second)
	defer tick.Stop()
	for range tick.C {
		w.sendHeartbeat()
	}
}

func (w *Worker) sendHeartbeat() {
	body := map[string]string{"id": w.workerID}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	resp, err := http.Post(w.apiURL+"/workers/heartbeat", "application/json", &buf)
	if err != nil {
		log.Printf("event=heartbeat_failed worker_id=%s error=%v", w.workerID, err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("event=heartbeat_rejected worker_id=%s status=%d", w.workerID, resp.StatusCode)
	}
}

// run request is the payload sent by the scheduler to post /run
type RunRequest struct {
	JobID      string `json:"job_id"`
	Type       string `json:"type"`
	Payload    string `json:"payload"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

func (w *Worker) handleRun(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "bad request", http.StatusBadRequest)
		return
	}
	log.Printf("event=job_exec_start job_id=%s worker_id=%s", req.JobID, w.workerID)
	result, err := w.exec.Run(req.JobID, req.Type, req.Payload, req.TimeoutSec)
	if err != nil {
		log.Printf("event=job_exec_error job_id=%s worker_id=%s error=%v", req.JobID, w.workerID, err)
		w.reportComplete(req.JobID, false, "", err.Error())
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("event=job_exec_done job_id=%s worker_id=%s success=%t", req.JobID, w.workerID, result.Success)
	w.reportComplete(req.JobID, result.Success, result.Output, result.Error)
	rw.WriteHeader(http.StatusOK)
}

func (w *Worker) reportComplete(jobID string, success bool, result, errMsg string) {
	body := map[string]interface{}{
		"success": success,
		"result":  result,
		"error":   errMsg,
	}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req, _ := http.NewRequest(http.MethodPost, w.apiURL+"/jobs/"+jobID+"/complete", &buf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("report complete failed: %v", err)
		return
	}
	defer resp.Body.Close()
}
