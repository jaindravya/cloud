package scheduler


import (
    "sync"
)


// queue is an in-memory FIFO queue of job ids
type Queue struct {
    mu    sync.Mutex
    items []string
}


// new queue creates a new queue
func NewQueue() *Queue {
    return &Queue{items: make([]string, 0)}
}


// enqueue adds a job id to the back of the queue
func (q *Queue) Enqueue(jobID string) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.items = append(q.items, jobID)
}


// dequeue removes and returns the front job id, or "" if empty
func (q *Queue) Dequeue() string {
    q.mu.Lock()
    defer q.mu.Unlock()
    if len(q.items) == 0 {
        return ""
    }
    jobID := q.items[0]
    q.items = q.items[1:]
    return jobID
}


// depth returns the number of job ids in the queue
func (q *Queue) Depth() int {
    q.mu.Lock()
    defer q.mu.Unlock()
    return len(q.items)
}


// remove removes the first occurrence of job id from the queue (e.g. on cancel)
func (q *Queue) Remove(jobID string) {
    q.mu.Lock()
    defer q.mu.Unlock()
    for i, id := range q.items {
        if id == jobID {
            q.items = append(q.items[:i], q.items[i+1:]...)
            return
        }
    }
}





