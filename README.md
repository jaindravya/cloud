# cloud – Distributed Job Scheduling Platform


**Go, C++, Docker, Systems Design 2025**


A lightweight distributed scheduler that accepts compute jobs from a REST API and dispatches them across multiple containerized worker nodes using a simple queue-based model.


- **Control logic in Go** – Registry of active workers, round-robin task assignment, basic heartbeat pings, and retry-on-failure handling for dropped tasks.
- **Minimal C++ execution module** – Used by workers to run CPU-bound jobs and return status/results, enabling predictable execution times and clean separation from scheduler logic.
- **Monitoring and scaling** – Monitoring endpoints and structured logs for job states, worker activity, and queue depth; simple scaling heuristics that recommend adding/removing workers when load shifts.


### in depth project description


Designed and implemented a distributed job scheduling platform that dispatches compute workloads across containerized worker nodes, mirroring patterns used in cloud-native systems. The control plane is implemented in Go and exposes a REST API for job submission, status, cancellation, and listing. Clients submit jobs with an optional type, payload, and per-job timeout; jobs enter an in-memory FIFO queue and are processed asynchronously. A dedicated scheduler loop runs inside the API process and continuously dequeues jobs, selects an idle worker using round-robin load balancing, and sends a run request to that worker over HTTP. The API maintains a registry of all active workers (id, endpoint, status, last heartbeat). Workers register on startup and periodically send heartbeat pings so the API can treat them as alive; workers that miss heartbeats beyond a configurable threshold are automatically reaped and removed from the registry, and any job they were running is put back on the queue so it can be retried on another worker. The scheduler includes retry-on-failure handling for dropped or failed dispatches (e.g. worker unreachable or returning an error). When a dispatch fails, the job is re-queued after an exponential backoff delay (2^retry seconds, capped) and retried up to a configurable limit before being marked failed, which avoids hammering a failing node and gives the system time to recover. Each worker is a separate process (or container) that listens for run requests. When it receives a job, it invokes a minimal C++ execution module (a standalone binary) with the job id, type, and payload. The C++ module runs the actual compute work and exits with a status and optional stdout; the worker then reports success or failure back to the API, which updates the job and frees the worker for the next job. This design gives predictable execution semantics, keeps scheduling logic in Go and heavy computation in C++, and makes it easy to swap or extend the execution layer. Monitoring is supported through a liveness endpoint (GET /health) and a readiness endpoint (GET /ready) that returns 503 until at least one worker is registered, so orchestrators like Kubernetes can wire probes correctly. A Prometheus-style metrics endpoint exposes queue depth, job counts by status, number of registered workers, and the maximum time since any worker’s last heartbeat. A separate JSON stats endpoint provides queue depth, worker count, jobs by status, success rate, and uptime for dashboards. Structured logging is used throughout (event=job_submitted, event=worker_heartbeat, event=scale_up, etc.) so that job lifecycle, worker activity, and scaling decisions can be traced and analyzed. Queue-depth-based auto-scaling heuristics observe the number of jobs waiting in the queue; when the queue exceeds a high threshold, the system can recommend or perform adding worker containers (e.g. via the Docker API), and when the queue stays below a low threshold for a sustained period, it can scale down. Thresholds and min/max worker counts are configurable, and scaling actions are logged. Production-oriented features include rate limiting on job submission (configurable requests per minute, 429 and Retry-After when exceeded), idempotent submission via an X-Idempotency-Key header so duplicate requests return the same job and clients can safely retry, pagination on job listing (limit and offset with total count), config validation at startup (e.g. MIN_WORKERS ≤ MAX_WORKERS, thresholds valid) with fail-fast and clear error messages, per-job execution timeouts so long-running jobs can be killed, request tracing via X-Request-ID on responses, and graceful shutdown so both the API and workers stop accepting new work and drain in-flight requests before exiting. The system is containerized with Docker and docker-compose for local development and deployment; the API and worker images are built from multi-stage Dockerfiles, and the API spec is written in OpenAPI 3.0 so it can be used in Swagger Editor for exploration and client generation.


### explaining it to a coworker


I built a small job scheduler so we can submit “compute jobs” (like “run this task with this input”) over an API and have them run on a pool of workers instead of on one machine. It’s the same idea as a mini cloud job queue. I wrote the brain of it in Go (the API and scheduler) and the part that actually runs each job in C++, and the workers run in Docker so we can scale them up or down. You post a job to the API, it goes into a queue, and a scheduler hands it off to an idle worker in round-robin order. Workers ping the API every so often so we know they’re alive; if one stops responding we mark it dead and put its job back in the queue so another worker can run it. Failed dispatches get retried a few times with backoff. I added things like rate limiting, idempotency keys, health/ready endpoints, and metrics so it behaves like something you’d run in production, not just a toy. So in a few sentences: it’s a distributed job queue with a Go API and scheduler, C++ for running the jobs, and Dockerized workers that you can scale based on how full the queue is.


## Features


- **REST API** – Submit jobs, get status, cancel, list jobs; worker registration and heartbeat; health and metrics
- **Scheduler** – In-memory FIFO queue; round-robin assignment to workers; retry on dispatch failure (up to 3 attempts)
- **Worker registry** – Active workers with heartbeat pings to refresh last-seen time
- **C++ execution** – Standalone binary for CPU-bound job execution; accepts `--job-id`, `--type`, `--payload`; returns via exit code and stdout
- **Structured logs** – `event=job_submitted`, `event=job_dispatched`, `event=job_completed`, `event=worker_heartbeat`, `event=scale_up`, `event=worker_stale`, etc.
- **Stale worker reaping** – Workers that miss heartbeats are pruned and their in-flight job is re-queued automatically.
- **Job timeout** – Optional `timeout_sec` per job so long-running jobs are killed; supports predictable execution.
- **Request tracing** – Responses include `X-Request-ID` (from request or generated) for tracing.
- **Richer metrics** – `job_total` by status, `worker_heartbeat_age_seconds`; Prometheus-ready.
- **Graceful shutdown** – API and worker drain in-flight work and shut down cleanly on SIGTERM/SIGINT.
- **Auto-scaler** – Queue-depth heuristics that recommend or perform adding/removing workers (optional Docker API)
- **Containerized** – Docker images for API and worker (Go + C++ runner)


### Production-ready extras


- **Liveness and readiness** – `GET /health` (liveness), `GET /ready` (readiness; 503 until workers register). Kubernetes-friendly.
- **Rate limiting** – Configurable limit on `POST /jobs` per minute; 429 with `Retry-After` when exceeded.
- **Idempotency** – `X-Idempotency-Key` header on job submit; duplicate requests return the same job (safe retries).
- **Dashboard stats** – `GET /stats` returns queue depth, worker count, jobs by status, success rate, uptime (JSON).
- **Pagination** – `GET /jobs?limit=50&offset=0` with `total` in the response.
- **Config validation** – Startup checks (e.g. `MIN_WORKERS` ≤ `MAX_WORKERS`, thresholds valid); fail fast with clear errors.
- **Exponential backoff** – Failed dispatches are re-queued after 2^retry seconds (capped) before retry.
- **API spec** – OpenAPI 3.0 in `api/openapi/openapi.yaml`; paste into [Swagger Editor](https://editor.swagger.io) to explore or generate clients.


## run and test (step by step)


**You do not need Docker.** With Go installed you can run the API and worker locally and test with curl. It works on **Windows and on a MacBook** without Docker—on Mac, use the “on macOS (MacBook)” section in [RUN.md](RUN.md). For a full walkthrough (what each command does, how to start the API and worker, and how to test), see **[RUN.md](RUN.md)**. RUN.md includes an **"All testing commands (PowerShell and Mac)"** section with copy-paste commands for both Windows PowerShell and Mac Terminal.


### Prerequisites


- Go 1.21+
- CMake and a C++17 compiler (for the execution module)
- Docker (optional, for containers and auto-scaling)


### Go


```bash
cd cloud
go mod tidy
go build -o api.exe ./cmd/api
go build -o worker.exe ./cmd/worker
# optional: build Go runner so worker can run jobs without building C++
go build -o runner.exe ./cmd/runner
```


Then start the API (`.\api.exe`), in another terminal set `EXECUTION_BINARY=.\runner.exe` and start the worker (`.\worker.exe`), and submit jobs with curl. See [RUN.md](RUN.md) for the full step-by-step guide.


### C++ runner


```bash
cd execution
cmake -B build .
cmake --build build
# binary at build/runner (or build/runner.exe on Windows)
```


### Docker


```bash
docker compose -f deploy/docker-compose.yaml build
docker compose -f deploy/docker-compose.yaml up -d
```


API: http://localhost:8080  
Submit a job: `curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d "{\"payload\":\"hello\"}"`


## Project layout


- `cmd/api` – REST API server (scheduler, queue, autoscaler)
- `cmd/worker` – Worker process (registers, heartbeats, runs jobs via C++ binary)
- `internal/api` – HTTP handlers
- `internal/scheduler` – Queue and scheduler loop (round-robin, retry-on-failure)
- `internal/loadbalancer` – Round-robin and least-connections selection
- `internal/autoscaler` – Queue-depth-based scaling (recommend or perform via Docker API)
- `internal/executor` – Invokes C++ binary from Go
- `internal/worker` – Worker HTTP server, registration, heartbeat loop
- `pkg/models` – Job, Worker, JobStore, WorkerRegistry
- `execution/` – C++ runner for CPU-bound jobs (CMake)
- `deploy/` – Dockerfiles and docker-compose
- `api/openapi/` – OpenAPI 3.0 spec


## Configuration (API)


| Variable | Description | Default |
|----------|-------------|---------|
| `QUEUE_THRESHOLD_HIGH` | Scale up when queue depth exceeds this | 10 |
| `QUEUE_THRESHOLD_LOW` | Scale down when queue depth below this | 2 |
| `MIN_WORKERS` | Minimum worker containers (autoscaler) | 1 |
| `MAX_WORKERS` | Maximum worker containers | 4 |
| `WORKER_IMAGE` | Docker image for scaling (e.g. `cloud-worker`) | – |
| `SCALE_DOWN_STABLE_SEC` | Seconds queue must be low before scale-down | 30 |
| `WORKER_HEARTBEAT_TIMEOUT_SEC` | Seconds without heartbeat before worker is reaped and its job re-queued | 90 |
| `RATE_LIMIT_JOBS_PER_MIN` | Max job submissions per minute (0 = no limit) | 120 |
| `IDEMPOTENCY_TTL_SEC` | How long idempotency keys are remembered (seconds) | 86400 |


When `WORKER_IMAGE` is set and Docker is available, the API uses the Docker API to start/stop worker containers based on queue depth.


## License


MIT





