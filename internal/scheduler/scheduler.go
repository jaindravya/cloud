package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"cloud/internal/loadbalancer"
	"cloud/pkg/models"
)

const maxDispatchRetries = 3

// scheduler assigns queued jobs to workers via http
type Scheduler struct {
	queue   *Queue
	store   *models.JobStore
	workers *models.WorkerRegistry
	client  *http.Client
	stop    chan struct{}
	done    sync.WaitGroup
}

// new creates a new scheduler
func New(queue *Queue, store *models.JobStore, workers *models.WorkerRegistry) *Scheduler {
	return &Scheduler{
		queue:   queue,
		store:   store,
		workers: workers,
		client:  &http.Client{Timeout: 30 * time.Second},
		stop:    make(chan struct{}),
	}
}

// start runs the scheduler loop in a goroutine
func (s *Scheduler) Start() {
	s.done.Add(1)
	go s.runLoop()
}

// stop signals the scheduler to stop and waits for it
func (s *Scheduler) Stop() {
	close(s.stop)
	s.done.Wait()
}

// on job complete is called when a worker finishes a job, marks the worker idle
func (s *Scheduler) OnJobComplete(jobID, workerID string) {
	w, ok := s.workers.Get(workerID)
	if !ok {
		return
	}
	w.Status = models.WorkerStatusIdle
	w.CurrentJobID = ""
	s.workers.Register(w)
}

// reap stale workers unregisters workers that missed heartbeat beyond threshold and re-queues their job
func (s *Scheduler) ReapStaleWorkers(threshold time.Duration) {
	now := time.Now()
	for _, w := range s.workers.List() {
		if now.Sub(w.LastHeartbeat) <= threshold {
			continue
		}
		if w.CurrentJobID != "" {
			job, ok := s.store.Get(w.CurrentJobID)
			if ok && job.Status == models.JobStatusRunning {
				job.Status = models.JobStatusQueued
				job.StartedAt = nil
				job.WorkerID = ""
				s.store.Update(job)
               s.queue.Enqueue(job.ID, job.Priority)
				log.Printf("event=worker_stale job_requeued worker_id=%s job_id=%s heartbeat_age_sec=%.0f", w.ID, job.ID, now.Sub(w.LastHeartbeat).Seconds())
			}
		}
		s.workers.Unregister(w.ID)
		log.Printf("event=worker_reaped worker_id=%s", w.ID)
	}
}

func (s *Scheduler) runLoop() {
	defer s.done.Done()
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
			s.tick()
		}
	}
}

func (s *Scheduler) tick() {
	jobID := s.queue.Dequeue()
	if jobID == "" {
		return
	}
	job, ok := s.store.Get(jobID)
	if !ok || job.Status == models.JobStatusCancelled {
		return
	}
	list := s.workers.List()
	worker := loadbalancer.SelectWorker(list, loadbalancer.RoundRobin)
	if worker == nil {
       s.queue.Enqueue(jobID, job.Priority)
		return
	}
	now := time.Now()
	job.Status = models.JobStatusRunning
	job.StartedAt = &now
	job.WorkerID = worker.ID
	s.store.Update(job)

	worker.Status = models.WorkerStatusBusy
	worker.CurrentJobID = jobID
	s.workers.Register(worker)

	log.Printf("event=worker_assigned worker_id=%s job_id=%s queue_depth=%d", worker.ID, jobID, s.queue.Depth())
	go s.dispatch(job, worker)
}

// run job request is sent to the worker
type RunJobRequest struct {
	JobID      string `json:"job_id"`
	Type       string `json:"type"`
	Payload    string `json:"payload"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
}

func (s *Scheduler) dispatch(job *models.Job, worker *models.Worker) {
	url := worker.Endpoint + "/run"
	body := RunJobRequest{JobID: job.ID, Type: job.Type, Payload: job.Payload, TimeoutSec: job.TimeoutSec}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		s.handleDispatchFailure(job, worker, err.Error())
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		s.handleDispatchFailure(job, worker, "worker returned "+resp.Status)
		return
	}
	log.Printf("event=job_dispatched job_id=%s worker_id=%s", job.ID, worker.ID)
   // worker will call back post /jobs/:id/complete when done
}

func (s *Scheduler) handleDispatchFailure(job *models.Job, worker *models.Worker, errMsg string) {
	s.OnJobComplete(job.ID, worker.ID)
	job.RetryCount++
	if job.RetryCount < maxDispatchRetries {
		job.Status = models.JobStatusQueued
		job.StartedAt = nil
		job.WorkerID = ""
		s.store.Update(job)
		backoffSec := 1 << uint(job.RetryCount)
		if backoffSec > 60 {
			backoffSec = 60
		}
		retryNum := job.RetryCount
       jobPriority := job.Priority
       go func(jobID string, priority int, delay time.Duration, retryCount int) {
			time.Sleep(delay)
           s.queue.Enqueue(jobID, priority)
			log.Printf("event=job_retry_queued job_id=%s retry_count=%d backoff_sec=%.0f error=%s", jobID, retryCount, delay.Seconds(), errMsg)
       }(job.ID, jobPriority, time.Duration(backoffSec)*time.Second, retryNum)
		return
	}
	job.Status = models.JobStatusFailed
	job.Error = errMsg
	s.store.Update(job)
	log.Printf("event=job_failed job_id=%s retry_count=%d error=%s", job.ID, job.RetryCount, errMsg)
}
