package autoscaler

import (
	"context"
	"log"
	"sync"
	"time"

	"cloud/internal/scheduler"
	"cloud/pkg/models"
)

// config holds autoscaling thresholds and limits
type Config struct {
	QueueThresholdHigh int           // scale up when queue depth > this
	QueueThresholdLow  int           // scale down when queue depth < this
	ScaleDownStableDur time.Duration // require low queue for this long before scale down
	MinWorkers         int
	MaxWorkers         int
	WorkerImage        string // docker image for worker (e.g. cloud-worker)
}

// scaler can scale worker containers via docker API
type Scaler interface {
	StartWorker(ctx context.Context) (containerID string, err error)
	StopWorker(ctx context.Context, containerID string) error
	WorkerContainerIDs(ctx context.Context) ([]string, error)
}

// auto scaler runs queue-depth-based scaling heuristics
type AutoScaler struct {
	cfg      Config
	queue    *scheduler.Queue
	workers  *models.WorkerRegistry
	scaler   Scaler
	lowSince time.Time
	mu       sync.Mutex
	stop     chan struct{}
	done     sync.WaitGroup
}

// new creates an autoscaler. if scaler is nil, no docker scaling is performed
func New(cfg Config, queue *scheduler.Queue, workers *models.WorkerRegistry, scaler Scaler) *AutoScaler {
	return &AutoScaler{
		cfg:     cfg,
		queue:   queue,
		workers: workers,
		scaler:  scaler,
		stop:    make(chan struct{}),
	}
}

// start runs the autoscaler loop
func (a *AutoScaler) Start() {
	if a.scaler == nil {
		return
	}
	a.done.Add(1)
	go a.loop()
}

// stop stops the autoscaler
func (a *AutoScaler) Stop() {
	close(a.stop)
	a.done.Wait()
}

func (a *AutoScaler) loop() {
	defer a.done.Done()
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-tick.C:
			a.tick()
		}
	}
}

func (a *AutoScaler) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	depth := a.queue.Depth()
	registeredCount := len(a.workers.List())

	containerIDs, err := a.scaler.WorkerContainerIDs(ctx)
	if err != nil {
		log.Printf("event=autoscaler_error action=list_containers error=%v", err)
		return
	}
	// count only containers we manage (same as worker pool for scaling)
	managedCount := len(containerIDs)

	if depth > a.cfg.QueueThresholdHigh && managedCount < a.cfg.MaxWorkers {
		_, err := a.scaler.StartWorker(ctx)
		if err != nil {
			log.Printf("event=scale_up_failed queue_depth=%d managed=%d registered=%d error=%v", depth, managedCount, registeredCount, err)
			return
		}
		log.Printf("event=scale_up queue_depth=%d managed=%d registered=%d action=added", depth, managedCount+1, registeredCount)
		return
	}
	if depth > a.cfg.QueueThresholdHigh && managedCount >= a.cfg.MaxWorkers {
		log.Printf("event=scale_recommend queue_depth=%d workers=%d recommendation=add_at_capacity", depth, managedCount)
	}

	// scale down, queue low for stable duration and above min
	a.mu.Lock()
	if depth < a.cfg.QueueThresholdLow {
		if a.lowSince.IsZero() {
			a.lowSince = time.Now()
		}
		if time.Since(a.lowSince) >= a.cfg.ScaleDownStableDur && managedCount > a.cfg.MinWorkers && len(containerIDs) > 0 {
			id := containerIDs[0]
			if err := a.scaler.StopWorker(ctx, id); err != nil {
				log.Printf("event=scale_down_failed queue_depth=%d workers=%d error=%v", depth, managedCount, err)
			} else {
				log.Printf("event=scale_down queue_depth=%d workers=%d action=removed", depth, managedCount-1)
			}
		}
	} else {
		a.lowSince = time.Time{}
	}
	a.mu.Unlock()
}
