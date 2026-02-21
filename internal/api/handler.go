package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud/internal/ratelimit"
	"cloud/internal/scheduler"
	"cloud/pkg/models"
)

func generateRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// handler implements the rest api for the job scheduling platform
type Handler struct {
	store       *models.JobStore
	queue       *scheduler.Queue
	workers     *models.WorkerRegistry
	sched       *scheduler.Scheduler
	startTime   time.Time
	rateLimiter rateLimiterInterface
	idem        *idempotency
}

type rateLimiterInterface interface {
	Allow() bool
}

// handler config optional config for production features
type HandlerConfig struct {
	StartTime         time.Time
	RateLimitPerMin   int
	IdempotencyTTLSec int
}

// new handler returns a new api handler. cfg can be nil for defaults
func NewHandler(store *models.JobStore, queue *scheduler.Queue, workers *models.WorkerRegistry, sched *scheduler.Scheduler, cfg *HandlerConfig) *Handler {
	h := &Handler{store: store, queue: queue, workers: workers, sched: sched}
	if cfg != nil {
		h.startTime = cfg.StartTime
		if cfg.StartTime.IsZero() {
			h.startTime = time.Now()
		}
		if cfg.RateLimitPerMin > 0 {
			h.rateLimiter = ratelimit.NewLimiter(cfg.RateLimitPerMin)
		}
		if cfg.IdempotencyTTLSec > 0 {
			h.idem = newIdempotency(time.Duration(cfg.IdempotencyTTLSec) * time.Second)
		}
	}
	if h.startTime.IsZero() {
		h.startTime = time.Now()
	}
	return h
}

// serve http routes requests to the appropriate handler
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = generateRequestID()
	}
	w.Header().Set("X-Request-ID", reqID)
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	switch {
	case path == "health" && r.Method == http.MethodGet:
		h.Health(w, r)
		return
	case path == "ready" && r.Method == http.MethodGet:
		h.Ready(w, r)
		return
	case path == "stats" && r.Method == http.MethodGet:
		h.Stats(w, r)
		return
	case path == "metrics" && r.Method == http.MethodGet:
		h.Metrics(w, r)
		return
   case path == "dashboard" && r.Method == http.MethodGet:
       h.Dashboard(w, r)
       return
   case path == "workers" && r.Method == http.MethodGet:
       h.ListWorkers(w, r)
       return
	case path == "jobs" && r.Method == http.MethodGet:
		h.ListJobs(w, r)
		return
	case path == "jobs" && r.Method == http.MethodPost:
		h.SubmitJob(w, r)
		return
	case len(parts) == 2 && parts[0] == "jobs" && r.Method == http.MethodGet:
		h.GetJob(w, r, parts[1])
		return
	case len(parts) == 2 && parts[0] == "jobs" && r.Method == http.MethodDelete:
		h.CancelJob(w, r, parts[1])
		return
	case len(parts) == 3 && parts[0] == "jobs" && parts[2] == "complete" && r.Method == http.MethodPost:
		h.CompleteJob(w, r, parts[1])
		return
	case path == "workers" && r.Method == http.MethodPost:
		h.RegisterWorker(w, r)
		return
	case path == "workers/heartbeat" && r.Method == http.MethodPost:
		h.Heartbeat(w, r)
		return
	default:
		http.NotFound(w, r)
	}
}

// health returns 200 ok for liveness (process alive)
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// ready returns 200 when ready to accept traffic (e.g. at least one worker), 503 otherwise
func (h *Handler) Ready(w http.ResponseWriter, _ *http.Request) {
	if len(h.workers.List()) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("no workers registered"))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// stats returns json summary for dashboards (queue, workers, jobs by status, uptime)
func (h *Handler) Stats(w http.ResponseWriter, _ *http.Request) {
	jobs := h.store.List("")
	byStatus := make(map[string]int)
	var completed, failed int
	for _, j := range jobs {
		byStatus[string(j.Status)]++
		if j.Status == models.JobStatusCompleted {
			completed++
		}
		if j.Status == models.JobStatusFailed {
			failed++
		}
	}
	total := len(jobs)
	successRate := 0.0
	if completed+failed > 0 {
		successRate = float64(completed) / float64(completed+failed) * 100
	}
	uptimeSec := time.Since(h.startTime).Seconds()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"queue_depth":      h.queue.Depth(),
		"workers":          len(h.workers.List()),
		"jobs_total":       total,
		"jobs_by_status":   byStatus,
		"success_rate_pct": successRate,
		"uptime_seconds":   uptimeSec,
	})
}

// metrics returns prometheus-style metrics (queue depth, worker count, jobs by status, heartbeat age)
func (h *Handler) Metrics(w http.ResponseWriter, _ *http.Request) {
	depth := h.queue.Depth()
	workers := h.workers.List()
	jobs := h.store.List("")
	statusCount := make(map[models.JobStatus]int)
	for _, j := range jobs {
		statusCount[j.Status]++
	}
	var maxHeartbeatAge float64
	now := time.Now()
	for _, w := range workers {
		age := now.Sub(w.LastHeartbeat).Seconds()
		if age > maxHeartbeatAge {
			maxHeartbeatAge = age
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("# HELP job_queue_depth number of jobs waiting in queue\n# TYPE job_queue_depth gauge\njob_queue_depth " + fmtInt(depth) + "\n"))
	_, _ = w.Write([]byte("# HELP workers_registered number of registered workers\n# TYPE workers_registered gauge\nworkers_registered " + fmtInt(len(workers)) + "\n"))
	_, _ = w.Write([]byte("# HELP job_total jobs by status\n# TYPE job_total gauge\n"))
	_, _ = w.Write([]byte("job_total{status=\"pending\"} " + fmtInt(statusCount[models.JobStatusPending]) + "\n"))
	_, _ = w.Write([]byte("job_total{status=\"queued\"} " + fmtInt(statusCount[models.JobStatusQueued]) + "\n"))
	_, _ = w.Write([]byte("job_total{status=\"running\"} " + fmtInt(statusCount[models.JobStatusRunning]) + "\n"))
	_, _ = w.Write([]byte("job_total{status=\"completed\"} " + fmtInt(statusCount[models.JobStatusCompleted]) + "\n"))
	_, _ = w.Write([]byte("job_total{status=\"failed\"} " + fmtInt(statusCount[models.JobStatusFailed]) + "\n"))
	_, _ = w.Write([]byte("job_total{status=\"cancelled\"} " + fmtInt(statusCount[models.JobStatusCancelled]) + "\n"))
	_, _ = w.Write([]byte("# HELP worker_heartbeat_age_seconds max seconds since last worker heartbeat\n# TYPE worker_heartbeat_age_seconds gauge\nworker_heartbeat_age_seconds " + fmtFloat(maxHeartbeatAge) + "\n"))
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b) - 1
	for n > 0 {
		b[i] = byte('0' + n%10)
		n /= 10
		i--
	}
	return string(b[i+1:])
}

// submit job handles post /jobs (rate limited, optional idempotency key)
func (h *Handler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	if h.rateLimiter != nil && !h.rateLimiter.Allow() {
		w.Header().Set("Retry-After", "60")
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}
	idemKey := r.Header.Get("X-Idempotency-Key")
	if idemKey != "" && h.idem != nil {
		if existingID, ok := h.idem.get(idemKey); ok {
			job, _ := h.store.Get(existingID)
			if job != nil {
				respondJSON(w, http.StatusOK, job)
				return
			}
		}
	}
	var req models.SubmitJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	priority := models.PriorityNormal
	if req.Priority != nil {
		p := *req.Priority
		if p < models.PriorityHigh {
			p = models.PriorityHigh
		}
		if p > models.PriorityLow {
			p = models.PriorityLow
		}
		priority = p
	}
	job := &models.Job{Type: req.Type, Payload: req.Payload, TimeoutSec: req.TimeoutSec, Priority: priority}
	job, err := h.store.Create(job)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if idemKey != "" && h.idem != nil {
		h.idem.set(idemKey, job.ID)
	}
	h.queue.Enqueue(job.ID, job.Priority)
	job.Status = models.JobStatusQueued
	h.store.Update(job)
	log.Printf("event=job_submitted job_id=%s queue_depth=%d", job.ID, h.queue.Depth())
	respondJSON(w, http.StatusAccepted, job)
}

// get job handles get /jobs/:id
func (h *Handler) GetJob(w http.ResponseWriter, _ *http.Request, id string) {
	job, ok := h.store.Get(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	respondJSON(w, http.StatusOK, job)
}

// cancel job handles delete /jobs/:id
func (h *Handler) CancelJob(w http.ResponseWriter, _ *http.Request, id string) {
	job, ok := h.store.Get(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if job.Status != models.JobStatusPending && job.Status != models.JobStatusQueued {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "job cannot be cancelled"})
		return
	}
	job.Status = models.JobStatusCancelled
	h.store.Update(job)
	h.queue.Remove(id)
	log.Printf("event=job_cancelled job_id=%s", id)
	respondJSON(w, http.StatusOK, job)
}

// list jobs handles get /jobs (pagination via limit and offset)
func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	status := models.JobStatus(r.URL.Query().Get("status"))
	limit := parseIntParam(r, "limit", 50, 1, 500)
	offset := parseIntParam(r, "offset", 0, 0, 10000)
	jobs := h.store.List(status)
	total := len(jobs)
	if offset >= total {
		jobs = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		jobs = jobs[offset:end]
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"jobs":   jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func parseIntParam(r *http.Request, key string, defaultVal, min, max int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	n, _ := strconv.Atoi(s)
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

// complete job handles post /jobs/:id/complete (callback from worker)
func (h *Handler) CompleteJob(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		Success bool   `json:"success"`
		Result  string `json:"result,omitempty"`
		Error   string `json:"error,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	job, ok := h.store.Get(id)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if job.Status != models.JobStatusRunning {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "job not running"})
		return
	}
	now := timeNow()
	job.FinishedAt = &now
	if req.Success {
		job.Status = models.JobStatusCompleted
		job.Result = req.Result
		log.Printf("event=job_completed job_id=%s worker_id=%s", id, job.WorkerID)
	} else {
		job.Status = models.JobStatusFailed
		job.Error = req.Error
		log.Printf("event=job_failed job_id=%s worker_id=%s error=%s", id, job.WorkerID, req.Error)
	}
	h.store.Update(job)
	h.sched.OnJobComplete(id, job.WorkerID)
	respondJSON(w, http.StatusOK, job)
}

// register worker handles post /workers (worker registration)
func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	worker, ok := h.workers.Get(req.ID)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "worker not found"})
		return
	}
	worker.LastHeartbeat = timeNow()
	h.workers.Register(worker)
	log.Printf("event=worker_heartbeat worker_id=%s", req.ID)
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// register worker handles post /workers
func (h *Handler) RegisterWorker(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       string `json:"id"`
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Endpoint == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "id and endpoint required"})
		return
	}
	if req.ID == "" {
		req.ID = models.MustGenerateID()
	}
	worker := &models.Worker{ID: req.ID, Endpoint: req.Endpoint, Status: models.WorkerStatusIdle}
	worker.LastHeartbeat = timeNow()
	h.workers.Register(worker)
	log.Printf("event=worker_registered worker_id=%s endpoint=%s", worker.ID, worker.Endpoint)
	respondJSON(w, http.StatusOK, worker)
}

func respondJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// dashboard serves the live dashboard html page
func (h *Handler) Dashboard(w http.ResponseWriter, _ *http.Request) {
   w.Header().Set("Content-Type", "text/html; charset=utf-8")
   w.WriteHeader(http.StatusOK)
   w.Write([]byte(DashboardHTML()))
}

// list workers handles get /workers (json list of registered workers)
func (h *Handler) ListWorkers(w http.ResponseWriter, _ *http.Request) {
   respondJSON(w, http.StatusOK, h.workers.List())
}

var timeNow = func() time.Time { return time.Now() }
