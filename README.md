# cloud


a small distributed job scheduler. submit compute jobs over a rest api; they run on a pool of workers. control logic in go, execution in c++, optional docker for scale.


---


## what it does


you post a job (with a payload, optional type and timeout). the api enqueues it. a scheduler assigns jobs to idle workers in round-robin order. workers run each job (via a tiny c++ binary or a go runner) and report back. higher-priority jobs go first.


**priority**: jobs can have optional priority `0` (high), `1` (normal, default), or `2` (low). if you don’t send `priority`, nothing changes for existing clients. the queue is a min-heap by priority with fifo tie-break, so urgent work gets dispatched before the rest.


workers register and send heartbeats; if one dies, its job is re-queued with the same priority and retried elsewhere. failed dispatches are retried with backoff. you get health/ready endpoints, metrics, rate limiting, idempotency keys, and graceful shutdown so it fits in a production-style setup.


---


## quick start (no docker)


you only need **go** to run and test.


```bash
cd cloud
go mod tidy
go build -o api ./cmd/api
go build -o worker ./cmd/worker
go build -o runner ./cmd/runner
```


**terminal 1: api**


```bash
./api
```


**terminal 2: worker**


```bash
export API_URL="http://localhost:8080"
export WORKER_ENDPOINT="http://localhost:9090"
export EXECUTION_BINARY="./runner"
./worker
```


**terminal 3: try it**


```bash
# check the api is up
curl http://localhost:8080/health


# submit a normal job (priority defaults to 1)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"hello"}'


# submit a high-priority job (dispatched first)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"urgent","priority":0}'


# submit a low-priority job (dispatched last)
curl -s -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"background work","priority":2}'


# check a job by id (use the "id" from any response above)
curl -s http://localhost:8080/jobs/<id>


# list all jobs (shows priority on each)
curl -s http://localhost:8080/jobs


# stats: queue depth, workers, jobs by status, success rate
curl -s http://localhost:8080/stats
```


open http://localhost:8080/dashboard in a browser to see the live dashboard: stats, job list with priority, workers, and a submit form that auto-refreshes every 2 seconds.

---
config is via env (e.g. `QUEUE_THRESHOLD_HIGH`, `MIN_WORKERS`, `RATE_LIMIT_JOBS_PER_MIN`).



