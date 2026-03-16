# cloud


a small distributed job scheduler with built-in job types. submit compute jobs over a rest api; they run on a pool of workers. control logic in go, execution in c++, optional docker for scale.


---


## what it does


you post a job (with a type, payload, optional priority and timeout). the api enqueues it. a scheduler assigns jobs to idle workers in round-robin order. workers run each job (via a go runner or c++ binary) and report back. higher-priority jobs go first.


### job types

the runner supports seven built-in job types out of the box:

| type | payload | what it does | example result |
|------|---------|--------------|----------------|
| `hash` | `{"input":"text"}` | computes sha-256 of the input string | `2cf24dba5fb0a30e...` |
| `prime` | `{"n":1000000}` | counts all primes up to n (sieve, max 100m) | `primes_up_to=1000000 count=78498 elapsed=12ms` |
| `fetch` | `{"url":"https://..."}` | fetches a url and returns status + body (blocks localhost/private ips) | `{"status":200,"content_length":1234,"body":"..."}` |
| `sleep` | `{"seconds":5}` | sleeps for n seconds (max 300), useful for testing | `slept for 5s` |
| `image-resize` | `{"input_path":"images/in.png","output_path":"images/out.png","width":320,"height":200}` | resizes an image; paths are files under data root (not URLs—put the image in ./data first) | `{"output_path":"images/out.png","width":320,"height":200,"format":"png"}` |
| `compress` | `{"input_paths":["reports/a.txt"],"output_path":"archives/reports.zip","format":"zip"}` | creates zip or tar.gz archives from files/dirs under data root | `{"format":"zip","output_path":"archives/reports.zip","file_count":2,"total_bytes":1024}` |
| `email` | `{"to":"user@example.com","subject":"...","text":"..."}` | sends an email via SMTP (requires SMTP env; supports STARTTLS and SMTPS) | `{"to":"user@example.com","subject":"...","status":"sent"}` |
| _(empty)_ | any string | echo: returns `OK:<payload>` (backwards compatible) | `OK:hello` |

if the client doesn't send a `type`, the runner echoes the payload like before, so existing clients keep working.

### priority

jobs can have optional priority `0` (high), `1` (normal, default), or `2` (low). if you don't send `priority`, nothing changes for existing clients. the queue is a min-heap by priority with fifo tie-break, so urgent work gets dispatched before the rest.

### reliability

workers register and send heartbeats; if one dies, its job is re-queued with the same priority and retried elsewhere. failed dispatches are retried with backoff. you get health/ready endpoints, metrics, rate limiting, idempotency keys, and graceful shutdown so it fits in a production-style setup.

### security

the `fetch` job type blocks requests to localhost, 127.0.0.1, ::1, and all private/link-local ip ranges (resolved via dns). only http and https schemes are allowed. response bodies are capped at 4kb. the `prime` type caps n at 100,000,000 and `sleep` caps at 300 seconds. file jobs (`image-resize`, `compress`) only allow relative paths under `RUNNER_DATA_ROOT` (default `./data`) and reject absolute paths and `..` traversal. image-resize does not fetch URLs—input_path and output_path must be paths to files already on disk under the data root.


---


## quick start (no docker)


you only need **go** to run and test. running locally means you can change code, rebuild the binary you changed, and restart that process to see updates—no docker rebuild needed.


```bash
cd cloud
go mod tidy
go build -o cloud-api ./cmd/api
go build -o cloud-worker ./cmd/worker
go build -o runner ./cmd/runner
```


**terminal 1: api**


if port 8080 is already in use (e.g. docker is still running), stop it first: `docker compose -f deploy/docker-compose.yaml down`. or find and kill the process using port 8080.


```bash
./cloud-api
```


**terminal 2+: workers**

each worker needs its own port. open a new terminal for each one and set `WORKER_PORT` + `WORKER_ENDPOINT` to a unique port:

```bash
export API_URL="http://localhost:8080"
export WORKER_PORT=9090
export WORKER_ENDPOINT="http://localhost:9090"
export EXECUTION_BINARY="./runner"
export RUNNER_DATA_ROOT="./data"
./cloud-worker
```

to add more workers, open another terminal and increment the port:

```bash
export API_URL="http://localhost:8080"
export WORKER_PORT=9091
export WORKER_ENDPOINT="http://localhost:9091"
export EXECUTION_BINARY="./runner"
export RUNNER_DATA_ROOT="./data"
./cloud-worker
```

you can run as many as you want (9092, 9093, ...). each one registers with the api and picks up jobs in parallel.

**optional: email jobs (SMTP).** to run `type: email` jobs, set SMTP env vars in the **same terminal** where you start the worker (before `./cloud-worker`), so the runner inherits them. required: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`. optional: `SMTP_FROM` (address the email is sent from; defaults to `SMTP_USER`), `SMTP_MODE` (`starttls` for port 587 or `smtps` for 465), `SMTP_TIMEOUT_SEC` (default 30). example for local:

```bash
export SMTP_HOST=smtp.gmail.com SMTP_PORT=587 SMTP_USER=you@gmail.com SMTP_PASS=your-app-password SMTP_FROM=you@gmail.com SMTP_MODE=starttls
./cloud-worker
```

for gmail, use an app password (google account → security → app passwords), not your normal password. for docker, add the same vars to the worker service in `deploy/docker-compose.yaml` (see commented SMTP lines there).


**next terminal: try it**


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

# image resize (input/output must be file paths under RUNNER_DATA_ROOT, e.g. ./data; not URLs—put the image file in ./data first or download it there)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"image-resize","payload":"{\"input_path\":\"images/in.png\",\"output_path\":\"images/out.png\",\"width\":320,\"height\":200}"}'

# compress files/directories into zip or tar.gz under data root
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"compress","payload":"{\"input_paths\":[\"reports\"],\"output_path\":\"archives/reports.zip\",\"format\":\"zip\"}"}'

# email (requires SMTP env in the worker terminal—see "optional: email jobs (SMTP)" above)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" \
  -d '{"type":"email","payload":"{\"to\":\"recipient@example.com\",\"subject\":\"Test\",\"text\":\"Hello from cloud\"}"}'

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

api config is via env (e.g. `QUEUE_THRESHOLD_HIGH`, `MIN_WORKERS`, `RATE_LIMIT_JOBS_PER_MIN`). worker config: `WORKER_PORT` (default 9090), `WORKER_ENDPOINT`, `EXECUTION_BINARY`, `RUNNER_DATA_ROOT` (default `./data`, docker uses `/app/data`). see `deploy/docker-compose.yaml` for the full list. for email jobs, see the **optional: email jobs (SMTP)** subsection under quick start (no docker).
