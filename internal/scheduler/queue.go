package scheduler

import (
   "container/heap"
   "sync"
)

// queue item is one entry in the priority queue (min-heap by priority, then sequence)
type queueItem struct {
   jobID    string
   priority int
   sequence uint64
}

// priority queue implements heap.interface; min by priority, then by sequence (fifo tie-break)
type priorityQueue []*queueItem

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
   if pq[i].priority != pq[j].priority {
       return pq[i].priority < pq[j].priority
   }
   return pq[i].sequence < pq[j].sequence
}
func (pq priorityQueue) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i] }
func (pq *priorityQueue) Push(x interface{}) { *pq = append(*pq, x.(*queueItem)) }
func (pq *priorityQueue) Pop() interface{} {
   old := *pq
   n := len(old)
   item := old[n-1]
   *pq = old[0 : n-1]
   return item
}

// queue is an in-memory priority queue of job ids (min-heap by priority, fifo tie-break).
// cancel is lazy: removed ids are skipped when popping.
type Queue struct {
   mu        sync.Mutex
   heap      priorityQueue
   cancelled map[string]struct{}
   sequence  uint64
   activeCount int
}

// new queue creates a new priority queue
func NewQueue() *Queue {
   q := &Queue{
       heap:      make(priorityQueue, 0),
       cancelled: make(map[string]struct{}),
   }
   heap.Init(&q.heap)
   return q
}

// enqueue adds a job id with the given priority (lower value = higher priority, dispatched first)
func (q *Queue) Enqueue(jobID string, priority int) {
   q.mu.Lock()
   defer q.mu.Unlock()
   q.sequence++
   heap.Push(&q.heap, &queueItem{jobID: jobID, priority: priority, sequence: q.sequence})
   q.activeCount++
}

// dequeue removes and returns the highest-priority job id (smallest priority value, oldest sequence on tie), or "" if empty.
// cancelled ids are skipped (lazy removal).
func (q *Queue) Dequeue() string {
   q.mu.Lock()
   defer q.mu.Unlock()
   for q.heap.Len() > 0 {
       item := heap.Pop(&q.heap).(*queueItem)
       if _, cancelled := q.cancelled[item.jobID]; cancelled {
           delete(q.cancelled, item.jobID)
           continue
       }
       q.activeCount--
       return item.jobID
   }
   return ""
}

// depth returns the number of job ids currently in the queue (excluding lazy-cancelled)
func (q *Queue) Depth() int {
   q.mu.Lock()
   defer q.mu.Unlock()
   return q.activeCount
}

// remove marks the job id as cancelled (lazy removal); it will be skipped when popped
func (q *Queue) Remove(jobID string) {
   q.mu.Lock()
   defer q.mu.Unlock()
   q.cancelled[jobID] = struct{}{}
   q.activeCount--
}
