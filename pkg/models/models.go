package models


import (
    "crypto/rand"
    "encoding/hex"
    "sync"
    "time"
)


// job status represents the lifecycle state of a job
type JobStatus string


const (
    JobStatusPending   JobStatus = "pending"
    JobStatusQueued    JobStatus = "queued"
    JobStatusRunning   JobStatus = "running"
    JobStatusCompleted JobStatus = "completed"
    JobStatusFailed    JobStatus = "failed"
    JobStatusCancelled JobStatus = "cancelled"
)

// job priority: lower value = higher priority (dispatched first)
const (
    PriorityHigh   = 0
    PriorityNormal = 1
    PriorityLow    = 2
)

// job represents a compute workload submitted to the platform
type Job struct {
    ID         string     `json:"id"`
    Type       string     `json:"type,omitempty"`
    Payload    string     `json:"payload"`
    Status     JobStatus  `json:"status"`
    Priority   int        `json:"priority,omitempty"` // 0=high, 1=normal, 2=low; default 1
    CreatedAt  time.Time  `json:"created_at"`
    StartedAt  *time.Time `json:"started_at,omitempty"`
    FinishedAt *time.Time `json:"finished_at,omitempty"`
    WorkerID   string     `json:"worker_id,omitempty"`
    Result     string     `json:"result,omitempty"`
    Error      string     `json:"error,omitempty"`
    RetryCount int        `json:"retry_count,omitempty"`
    TimeoutSec int        `json:"timeout_sec,omitempty"`
}


// submit job request is the body for post /jobs
type SubmitJobRequest struct {
    Type       string `json:"type,omitempty"`
    Payload    string `json:"payload"`
    TimeoutSec int    `json:"timeout_sec,omitempty"`
    Priority   *int   `json:"priority,omitempty"` // optional; 0=high, 1=normal, 2=low; default 1
}


// worker status represents worker availability
type WorkerStatus string


const (
    WorkerStatusIdle WorkerStatus = "idle"
    WorkerStatusBusy WorkerStatus = "busy"
)


// worker represents a containerized worker node
type Worker struct {
    ID            string        `json:"id"`
    Endpoint      string        `json:"endpoint"`
    Status        WorkerStatus  `json:"status"`
    CurrentJobID  string        `json:"current_job_id,omitempty"`
    LastHeartbeat time.Time     `json:"last_heartbeat"`
}


// job store holds jobs in memory
type JobStore struct {
    jobs map[string]*Job
    mu   sync.RWMutex
}


// worker registry holds registered workers
type WorkerRegistry struct {
    workers map[string]*Worker
    mu      sync.RWMutex
}


// new job store creates an in-memory job store
func NewJobStore() *JobStore {
    return &JobStore{jobs: make(map[string]*Job)}
}


// create creates a new job and returns it
func (s *JobStore) Create(job *Job) (*Job, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if job.ID == "" {
        job.ID = mustGenerateID()
    }
    job.CreatedAt = time.Now()
    job.Status = JobStatusPending
    s.jobs[job.ID] = job
    return job, nil
}


// get returns a job by id
func (s *JobStore) Get(id string) (*Job, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    j, ok := s.jobs[id]
    return j, ok
}


// update updates a job (caller must hold a reference, we replace by id)
func (s *JobStore) Update(job *Job) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.jobs[job.ID] = job
}


// list returns all jobs (optional filter by status)
func (s *JobStore) List(status JobStatus) []*Job {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var out []*Job
    for _, j := range s.jobs {
        if status == "" || j.Status == status {
            out = append(out, j)
        }
    }
    return out
}


func mustGenerateID() string {
    b := make([]byte, 8)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}


// must generate id generates a new unique id (exported for worker registration)
func MustGenerateID() string {
    return mustGenerateID()
}


// new worker registry creates a worker registry
func NewWorkerRegistry() *WorkerRegistry {
    return &WorkerRegistry{workers: make(map[string]*Worker)}
}


// register adds or updates a worker
func (r *WorkerRegistry) Register(w *Worker) {
    r.mu.Lock()
    defer r.mu.Unlock()
    w.LastHeartbeat = time.Now()
    r.workers[w.ID] = w
}


// unregister removes a worker
func (r *WorkerRegistry) Unregister(id string) {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.workers, id)
}


// get returns a worker by id
func (r *WorkerRegistry) Get(id string) (*Worker, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    w, ok := r.workers[id]
    return w, ok
}


// list returns all workers
func (r *WorkerRegistry) List() []*Worker {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make([]*Worker, 0, len(r.workers))
    for _, w := range r.workers {
        out = append(out, w)
    }
    return out
}





