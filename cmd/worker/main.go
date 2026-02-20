package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud/internal/executor"
	worker "cloud/internal/worker"
)

func main() {
	apiURL := getEnv("API_URL", "http://localhost:8080")
	workerID := getEnv("WORKER_ID", "")
	execPath := getEnv("EXECUTION_BINARY", "/app/runner")

	execRunner := executor.NewRunner(execPath)
	w := worker.New(apiURL, workerID, execRunner)
	if err := w.Start(); err != nil {
		log.Fatal(err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Println("worker shutting down, draining current job...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := w.Shutdown(ctx); err != nil {
		log.Printf("worker shutdown: %v", err)
	}
	log.Println("worker stopped")
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
