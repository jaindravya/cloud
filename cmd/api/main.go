package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"cloud/internal/api"
	"cloud/internal/autoscaler"
	"cloud/internal/scheduler"
	"cloud/pkg/models"
)

func main() {
	startTime := time.Now()

	// config with validation
	queueThresholdHigh := getEnvInt("QUEUE_THRESHOLD_HIGH", 10)
	queueThresholdLow := getEnvInt("QUEUE_THRESHOLD_LOW", 2)
	minWorkers := getEnvInt("MIN_WORKERS", 1)
	maxWorkers := getEnvInt("MAX_WORKERS", 4)
	validateConfig(queueThresholdHigh, queueThresholdLow, minWorkers, maxWorkers)

	store := models.NewJobStore()
	queue := scheduler.NewQueue()
	workerRegistry := models.NewWorkerRegistry()
	sched := scheduler.New(queue, store, workerRegistry)
	sched.Start()
	defer sched.Stop()

	var scaler autoscaler.Scaler
	if img := os.Getenv("WORKER_IMAGE"); img != "" {
		if s, err := autoscaler.NewDockerScaler(img); err != nil {
			log.Printf("autoscaler: Docker unavailable: %v", err)
		} else {
			scaler = s
		}
	}
	cfg := autoscaler.Config{
		QueueThresholdHigh: queueThresholdHigh,
		QueueThresholdLow:  queueThresholdLow,
		ScaleDownStableDur: time.Duration(getEnvInt("SCALE_DOWN_STABLE_SEC", 30)) * time.Second,
		MinWorkers:         minWorkers,
		MaxWorkers:         maxWorkers,
		WorkerImage:        os.Getenv("WORKER_IMAGE"),
	}
	as := autoscaler.New(cfg, queue, workerRegistry, scaler)
	as.Start()
	defer as.Stop()

	// stale worker reaper
	heartbeatTimeout := time.Duration(getEnvInt("WORKER_HEARTBEAT_TIMEOUT_SEC", 90)) * time.Second
	go func() {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		for range tick.C {
			sched.ReapStaleWorkers(heartbeatTimeout)
		}
	}()

	apiCfg := &api.HandlerConfig{
		StartTime:         startTime,
		RateLimitPerMin:   getEnvInt("RATE_LIMIT_JOBS_PER_MIN", 120),
		IdempotencyTTLSec: getEnvInt("IDEMPOTENCY_TTL_SEC", 86400),
	}
	handler := api.NewHandler(store, queue, workerRegistry, sched, apiCfg)
	srv := &http.Server{Addr: ":8080", Handler: handler}
	go func() {
		log.Println("API listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
	log.Println("API stopped")
}

func getEnvInt(key string, defaultVal int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultVal
	}
	n, _ := strconv.Atoi(s)
	if n <= 0 {
		return defaultVal
	}
	return n
}

func validateConfig(thresholdHigh, thresholdLow, minWorkers, maxWorkers int) {
	if minWorkers > maxWorkers {
		log.Fatalf("config invalid: MIN_WORKERS (%d) must be <= MAX_WORKERS (%d)", minWorkers, maxWorkers)
	}
	if thresholdLow >= thresholdHigh {
		log.Fatalf("config invalid: QUEUE_THRESHOLD_LOW (%d) must be < QUEUE_THRESHOLD_HIGH (%d)", thresholdLow, thresholdHigh)
	}
	if thresholdHigh <= 0 || thresholdLow < 0 {
		log.Fatal("config invalid: queue thresholds must be non-negative and high > 0")
	}
}
