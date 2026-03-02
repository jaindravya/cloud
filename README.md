# cloud


a small distributed job scheduler with built-in job types. submit compute jobs over a rest api; they run on a pool of workers. control logic in go, execution in c++, optional docker for scale.


---


## what it does


you post a job (with a type, payload, optional priority and timeout). the api enqueues it. a scheduler assigns jobs to idle workers in round-robin order. workers run each job (via a go runner or c++ binary) and report back. higher-priority jobs go first.


### job types

the runner supports four built-in job types out of the box:

| type | payload | what it does | example result |
|------|---------|--------------|----------------|
| `hash` | `{"input":"text"}` | computes sha-256 of the input string | `2cf24dba5fb0a30e...` |
| `prime` | `{"n":1000000}` | counts all primes up to n (sieve, max 100m) | `primes_up_to=1000000 count=78498 elapsed=12ms` |
| `fetch` | `{"url":"https://..."}` | fetches a url and returns status + body (blocks localhost/private ips) | `{"status":200,"content_length":1234,"body":"..."}` |
| `sleep` | `{"seconds":5}` | sleeps for n seconds (max 300), useful for testing | `slept for 5s` |
| _(empty)_ | any string | echo: returns `OK:<payload>` (backwards compatible) | `OK:hello` |

if the client doesn't send a `type`, the runner echoes the payload like before, so existing clients keep working.

### priority

jobs can have optional priority `0` (high), `1` (normal, default), or `2` (low). if you don't send `priority`, nothing changes for existing clients. the queue is a min-heap by priority with fifo tie-break, so urgent work gets dispatched before the rest.

### reliability

workers register and send heartbeats; if one dies, its job is re-queued with the same priority and retried elsewhere. failed dispatches are retried with backoff. you get health/ready endpoints, metrics, rate limiting, idempotency keys, and graceful shutdown so it fits in a production-style setup.

### security

the `fetch` job type blocks requests to localhost, 127.0.0.1, ::1, and all private/link-local ip ranges (resolved via dns). only http and https schemes are allowed. response bodies are capped at 4kb. the `prime` type caps n at 100,000,000 and `sleep` caps at 300 seconds to prevent resource abuse.


---


## quick start (no docker)


you only need **go** to run and test.


```bash
cd cloud
go mod tidy
go build -o cloud-api ./cmd/api
go build -o cloud-worker ./cmd/worker
go build -o runner ./cmd/runner
```


**terminal 1: api**


```bash
./cloud-api
```


**terminal 2: worker**


```bash
export API_URL="http://localhost:8080"
export WORKER_ENDPOINT="http://localhost:9090"
export EXECUTION_BINARY="./runner"
./cloud-worker
```


**terminal 3: try it**


```bash
# hash some text (high priority)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"hash","payload":"{\"input\":\"hello world\"}","priority":0}'

# count primes up to 1 million (normal priority)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"prime","payload":"{\"n\":1000000}"}'

# fetch a url (low priority)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"fetch","payload":"{\"url\":\"https://httpbin.org/get\"}","priority":2}'

# sleep for 5 seconds (test timeouts and dashboard)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"sleep","payload":"{\"seconds\":5}"}'

# plain echo (backwards compatible, no type needed)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"payload":"hello"}'

# check a job by id
curl -s http://localhost:8080/jobs/<id>

# list all jobs
curl -s http://localhost:8080/jobs

# stats
curl -s http://localhost:8080/stats
```


open http://localhost:8080/dashboard in a browser for the live dashboard: stats, job list with type and priority, workers, and a submit form with type/priority selectors that auto-refreshes every 2 seconds.

---

## run with docker

you don't need go installed for this. just docker.

```bash
cd cloud
docker compose -f deploy/docker-compose.yaml build
docker compose -f deploy/docker-compose.yaml up -d
```

that starts the api on port 8080 and two workers. test with:

```bash
curl http://localhost:8080/health
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"hash","payload":"{\"input\":\"hello from docker\"}","priority":0}'
curl -s http://localhost:8080/stats
```

open http://localhost:8080/dashboard for the live ui.

to stop everything:

```bash
docker compose -f deploy/docker-compose.yaml down
```

---

config is via env (e.g. `QUEUE_THRESHOLD_HIGH`, `MIN_WORKERS`, `RATE_LIMIT_JOBS_PER_MIN`). see `deploy/docker-compose.yaml` for the full list.
