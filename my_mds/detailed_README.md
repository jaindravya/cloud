# cloud – distributed job scheduling platform


**go, c++, docker, systems design 2025**


a lightweight distributed scheduler that accepts compute jobs from a rest api and dispatches them across multiple containerized worker nodes using a simple queue-based model.


- **control logic in go** – registry of active workers, round-robin task assignment, basic heartbeat pings, and retry-on-failure handling for dropped tasks.
- **minimal c++ execution module** – used by workers to run cpu-bound jobs and return status/results, enabling predictable execution times and clean separation from scheduler logic.
- **monitoring and scaling** – monitoring endpoints and structured logs for job states, worker activity, and queue depth; simple scaling heuristics that recommend adding/removing workers when load shifts.


### in depth project description


designed and implemented a distributed job scheduling platform that dispatches compute workloads across containerized worker nodes, mirroring patterns used in cloud-native systems. the control plane is implemented in go and exposes a rest api for job submission, status, cancellation, and listing. clients submit jobs with an optional type, payload, per-job timeout, and priority (0=high, 1=normal, 2=low; default normal); jobs enter an in-memory priority queue (min-heap by priority with fifo tie-break) and are processed asynchronously, so higher-priority jobs are dispatched first. a dedicated scheduler loop runs inside the api process and continuously dequeues jobs, selects an idle worker using round-robin load balancing, and sends a run request to that worker over http. the api maintains a registry of all active workers (id, endpoint, status, last heartbeat). workers register on startup and periodically send heartbeat pings so the api can treat them as alive; workers that miss heartbeats beyond a configurable threshold are automatically reaped and removed from the registry, and any job they were running is put back on the queue so it can be retried on another worker. the scheduler includes retry-on-failure handling for dropped or failed dispatches (e.g. worker unreachable or returning an error). when a dispatch fails, the job is re-queued after an exponential backoff delay (2^retry seconds, capped) and retried up to a configurable limit before being marked failed, which avoids hammering a failing node and gives the system time to recover. each worker is a separate process (or container) that listens for run requests. when it receives a job, it invokes a runner binary (go or c++) with the job id, type, and payload. the runner supports built-in job types: hash (sha-256 of input text), prime (sieve of eratosthenes up to n, capped at 100m), fetch (http get with ssrf protection that blocks localhost, private ips, and link-local addresses; response body capped at 4kb), and sleep (simulated workload, capped at 300 seconds). for unknown or empty types it falls back to echo for backwards compatibility. the runner exits with a status and stdout; the worker then reports success or failure back to the api, which updates the job and frees the worker for the next job. this design gives predictable execution semantics, keeps scheduling logic in go and computation in the runner, and makes it easy to add new job types. monitoring is supported through a liveness endpoint (get /health) and a readiness endpoint (get /ready) that returns 503 until at least one worker is registered, so orchestrators like kubernetes can wire probes correctly. a prometheus-style metrics endpoint exposes queue depth, job counts by status, number of registered workers, and the maximum time since any worker's last heartbeat. a separate json stats endpoint provides queue depth, worker count, jobs by status, success rate, and uptime for dashboards. structured logging is used throughout (event=job_submitted, event=worker_heartbeat, event=scale_up, etc.) so that job lifecycle, worker activity, and scaling decisions can be traced and analyzed. queue-depth-based auto-scaling heuristics observe the number of jobs waiting in the queue; when the queue exceeds a high threshold, the system can recommend or perform adding worker containers (e.g. via the docker api), and when the queue stays below a low threshold for a sustained period, it can scale down. thresholds and min/max worker counts are configurable, and scaling actions are logged. production-oriented features include rate limiting on job submission (configurable requests per minute, 429 and retry-after when exceeded), idempotent submission via an x-idempotency-key header so duplicate requests return the same job and clients can safely retry, pagination on job listing (limit and offset with total count), config validation at startup (e.g. min_workers ≤ max_workers, thresholds valid) with fail-fast and clear error messages, per-job execution timeouts so long-running jobs can be killed, request tracing via x-request-id on responses, and graceful shutdown so both the api and workers stop accepting new work and drain in-flight requests before exiting. the system is containerized with docker and docker-compose for local development and deployment; the api and worker images are built from multi-stage dockerfiles, and the api spec is written in openapi 3.0 so it can be used in swagger editor for exploration and client generation.


### explaining it to a coworker


i built a small job scheduler so we can submit "compute jobs" (like "hash this text", "count primes up to n", "fetch this url") over an api and have them run on a pool of workers instead of on one machine. it's the same idea as a mini cloud job queue. i wrote the control logic in go (the api and scheduler) and the runner supports both go and c++, and the workers run in docker so we can scale them up or down. you post a job to the api with a type (hash, prime, fetch, sleep, or just plain echo), it goes into a priority queue, and a scheduler hands it off to an idle worker in round-robin order. workers ping the api every so often so we know they're alive; if one stops responding we mark it dead and put its job back in the queue so another worker can run it. failed dispatches get retried a few times with backoff. the fetch job type has ssrf protection built in: it blocks requests to localhost, private ips, and link-local addresses. i added things like rate limiting, idempotency keys, health/ready endpoints, metrics, and a live dashboard so it behaves like something you'd run in production, not just a toy. so in a few sentences: it's a distributed job queue with real job types (hashing, prime counting, http fetching, simulated workloads), a go api and scheduler, go/c++ runners, and dockerized workers that you can scale based on how full the queue is.


## features


- **rest api** – submit jobs, get status, cancel, list jobs; worker registration and heartbeat; health and metrics
- **scheduler** – priority queue (optional high/normal/low; min-heap, fifo tie-break); round-robin assignment to workers; retry on dispatch failure (up to 3 attempts)
- **worker registry** – active workers with heartbeat pings to refresh last-seen time
- **built-in job types** – `hash` (sha-256), `prime` (sieve up to n), `fetch` (http get with ssrf protection), `sleep` (simulated workload). falls back to echo for unknown types so existing clients keep working.
- **c++ execution** – standalone binary for cpu-bound job execution; accepts `--job-id`, `--type`, `--payload`; supports hash, prime, and sleep types natively; returns via exit code and stdout
- **structured logs** – `event=job_submitted`, `event=job_dispatched`, `event=job_completed`, `event=worker_heartbeat`, `event=scale_up`, `event=worker_stale`, etc.
- **stale worker reaping** – workers that miss heartbeats are pruned and their in-flight job is re-queued automatically.
- **job timeout** – optional `timeout_sec` per job so long-running jobs are killed; supports predictable execution.
- **request tracing** – responses include `x-request-id` (from request or generated) for tracing.
- **richer metrics** – `job_total` by status, `worker_heartbeat_age_seconds`; prometheus-ready.
- **graceful shutdown** – api and worker drain in-flight work and shut down cleanly on sigterm/sigint.
- **auto-scaler** – queue-depth heuristics that recommend or perform adding/removing workers (optional docker api)
- **containerized** – docker images for api and worker (go + c++ runner)


### production-ready extras


- **liveness and readiness** – `get /health` (liveness), `get /ready` (readiness; 503 until workers register). kubernetes-friendly.
- **rate limiting** – configurable limit on `post /jobs` per minute; 429 with `retry-after` when exceeded.
- **idempotency** – `x-idempotency-key` header on job submit; duplicate requests return the same job (safe retries).
- **dashboard stats** – `get /stats` returns queue depth, worker count, jobs by status, success rate, uptime (json).
- **job priority** – optional `priority` on submit (0=high, 1=normal, 2=low); default 1 so existing clients are unchanged; high-priority jobs are dispatched before lower-priority ones; priority is preserved when jobs are re-queued (e.g. worker died or retry).
- **pagination** – `get /jobs?limit=50&offset=0` with `total` in the response.
- **config validation** – startup checks (e.g. `min_workers` ≤ `max_workers`, thresholds valid); fail fast with clear errors.
- **exponential backoff** – failed dispatches are re-queued after 2^retry seconds (capped) before retry.
- **api spec** – openapi 3.0 in `api/openapi/openapi.yaml`; paste into [swagger editor](https://editor.swagger.io) to explore or generate clients.


## run and test (step by step)


**you do not need docker.** with go installed you can run the api and worker locally and test with curl. it works on **windows and on a macbook** without docker—on mac, use the "on macos (macbook)" section in [run.md](RUN.md). for a full walkthrough (what each command does, how to start the api and worker, and how to test), see **[run.md](RUN.md)**. run.md includes an **"all testing commands (powershell and mac)"** section with copy-paste commands for both windows powershell and mac terminal.


### prerequisites


- go 1.21+
- cmake and a c++17 compiler (for the execution module)
- docker (optional, for containers and auto-scaling)


### go


```bash
cd cloud
go mod tidy
go build -o api.exe ./cmd/api
go build -o worker.exe ./cmd/worker
# optional: build Go runner so worker can run jobs without building C++
go build -o runner.exe ./cmd/runner
```


then start the api (`.\api.exe`), in another terminal set `EXECUTION_BINARY=.\runner.exe` and start the worker (`.\worker.exe`), and submit jobs with curl. see [run.md](RUN.md) for the full step-by-step guide.


### c++ runner


```bash
cd execution
cmake -B build .
cmake --build build
# binary at build/runner (or build/runner.exe on Windows)
```


### docker


```bash
docker compose -f deploy/docker-compose.yaml build
docker compose -f deploy/docker-compose.yaml up -d
```


api: http://localhost:8080  
submit a job: `curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d "{\"payload\":\"hello\"}"`


## project layout


- `cmd/api` – rest api server (scheduler, queue, autoscaler)
- `cmd/worker` – worker process (registers, heartbeats, runs jobs via c++ binary)
- `internal/api` – http handlers
- `internal/scheduler` – queue and scheduler loop (round-robin, retry-on-failure)
- `internal/loadbalancer` – round-robin and least-connections selection
- `internal/autoscaler` – queue-depth-based scaling (recommend or perform via docker api)
- `internal/executor` – invokes c++ binary from go
- `internal/worker` – worker http server, registration, heartbeat loop
- `pkg/models` – job, worker, jobstore, workerregistry
- `execution/` – c++ runner for cpu-bound jobs (cmake)
- `deploy/` – dockerfiles and docker-compose
- `api/openapi/` – openapi 3.0 spec


## configuration (api)


| variable | description | default |
|----------|-------------|---------|
| `QUEUE_THRESHOLD_HIGH` | scale up when queue depth exceeds this | 10 |
| `QUEUE_THRESHOLD_LOW` | scale down when queue depth below this | 2 |
| `MIN_WORKERS` | minimum worker containers (autoscaler) | 1 |
| `MAX_WORKERS` | maximum worker containers | 4 |
| `WORKER_IMAGE` | docker image for scaling (e.g. `cloud-worker`) | – |
| `SCALE_DOWN_STABLE_SEC` | seconds queue must be low before scale-down | 30 |
| `WORKER_HEARTBEAT_TIMEOUT_SEC` | seconds without heartbeat before worker is reaped and its job re-queued | 90 |
| `RATE_LIMIT_JOBS_PER_MIN` | max job submissions per minute (0 = no limit) | 120 |
| `IDEMPOTENCY_TTL_SEC` | how long idempotency keys are remembered (seconds) | 86400 |


## configuration (worker)


| variable | description | default |
|----------|-------------|---------|
| `API_URL` | url of the api server | `http://localhost:8080` |
| `WORKER_ID` | unique id for this worker (auto-generated if empty) | – |
| `WORKER_PORT` | port the worker listens on for job requests | `9090` |
| `WORKER_ENDPOINT` | url the api uses to reach this worker | `http://localhost:9090` |
| `EXECUTION_BINARY` | path to the runner binary | `/app/runner` |


to run multiple workers locally, give each one a different `WORKER_PORT` and matching `WORKER_ENDPOINT`.


when `WORKER_IMAGE` is set and docker is available, the api uses the docker api to start/stop worker containers based on queue depth.


## license


mit
