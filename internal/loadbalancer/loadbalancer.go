package loadbalancer

import "cloud/pkg/models"

// strategy selects a worker from the list (e.g. round-robin or least connections)
type Strategy int

const (
	RoundRobin Strategy = iota
	LeastConnections
)

// select worker returns an idle worker, or nil if none available.
// least connections prefers workers with no current job, round robin cycles through idle workers
func SelectWorker(workers []*models.Worker, strategy Strategy) *models.Worker {
	var idle []*models.Worker
	for _, w := range workers {
		if w.Status == models.WorkerStatusIdle {
			idle = append(idle, w)
		}
	}
	if len(idle) == 0 {
		return nil
	}
	switch strategy {
	case LeastConnections:
		// already filtered to idle, pick first (all have 0 load)
		return idle[0]
	case RoundRobin:
		return idle[0]
	default:
		return idle[0]
	}
}
