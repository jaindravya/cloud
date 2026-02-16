# how to run and test cloud (step by step)


you can run the project **without Docker**. you only need **Go** installed. Docker is optional.


---


## what are the Docker files for?


the `deploy/` folder (Dockerfile.api, Dockerfile.worker, docker-compose.yaml) is for people who **choose** to run the project inside containers. with Docker installed, you can run one command and get the API plus two workers without building or running anything by hand. **you do not need Docker to run or test the project.** the same app runs with just Go: you build the API and worker on your machine and run them. so use the Docker files only if you already have Docker and want that workflow; otherwise ignore them and follow the Windows or Mac guide below.


---


## Windows – detailed guide (start here on your laptop)


follow these steps in order. use **PowerShell** (right‑click Start → Windows PowerShell or “Terminal”) or the terminal inside Cursor / VS Code.


### 0. Install Go (required – “go is not recognized” means do this first)


if you see **“go is not recognized”** or **“The term 'go' is not recognized”**, Go is not installed or not on your PATH. do this:


1. **Download Go for Windows**
   - Open in your browser: **https://go.dev/dl/**
   - Under “Microsoft Windows”, click the **.msi** link (e.g. `go1.22.4.windows-amd64.msi`). that downloads the installer.


2. **Run the installer**
   - Double‑click the downloaded `.msi` file.
   - Click “Next” through the steps (default install path is fine).
   - Finish the install.


3. **Use a new terminal**
   - **Close** the terminal where you saw “go is not recognized” (or close Cursor/VS Code and reopen it).
   - Open a **new** terminal in your project folder.


4. **Check that Go is available**


```powershell
go version
```


you should see something like `go version go1.22.x windows/amd64`. if you still get “not recognized”, try:
- fully closing and reopening Cursor (or your editor), then run `go version` again, or
- opening a **new** PowerShell from the Start menu, then `cd c:\Users\drjain\Desktop\cloud` and run `go version`.


once `go version` works, continue with step 1 below.


---


### 1. Open the project folder


```powershell
cd c:\Users\drjain\Desktop\cloud
```


**what this does:** changes into your project folder so all commands run in the right place. if your project is somewhere else (e.g. `C:\Projects\cloud`), use that path instead.


---


### 2. Download dependencies


```powershell
go mod tidy
```


**what this does:** reads `go.mod`, downloads any required packages, and updates `go.sum`. run this once per machine (or after pulling changes that add dependencies). you may see it download a few modules; that’s normal.


---


### 3. Build the three programs


run these one after the other:


```powershell
go build -o api.exe .\cmd\api
go build -o worker.exe .\cmd\worker
go build -o runner.exe .\cmd\runner
```


**what this does:**


- **api.exe** – the REST API and scheduler (listens on port 8080).
- **worker.exe** – the worker that registers with the API and runs jobs (listens on port 9090).
- **runner.exe** – a small helper that “runs” each job (prints `OK:<payload>`). the worker calls this so you don’t need to build the C++ binary.


if all three finish with no errors, you’re ready to run.


---


### 4. Start the API (first terminal)


in this terminal, run:


```powershell
.\api.exe
```


**what this does:** starts the API server. it will keep running and show:


```
API listening on :8080
```


**leave this terminal open.** do not close it. the API must stay running for the rest of the steps.


---


### 5. Start the worker (second terminal)


open a **new** terminal (new tab or new PowerShell window). then run:


```powershell
cd c:\Users\drjain\Desktop\cloud
$env:API_URL = "http://localhost:8080"
$env:WORKER_ENDPOINT = "http://localhost:9090"
$env:EXECUTION_BINARY = "c:\Users\drjain\Desktop\cloud\runner.exe"
.\worker.exe
```


**what this does:**


- `cd ...` – go to the project folder again (this terminal starts in a different place).
- `$env:API_URL` – tells the worker where the API is so it can register and report results.
- `$env:WORKER_ENDPOINT` – tells the API how to reach this worker (localhost:9090).
- `$env:EXECUTION_BINARY` – **use the full path** to `runner.exe` so the worker can find it (adjust if your project path is different).
- `.\worker.exe` – starts the worker.


you should see something like:


```
Registered with API as worker-xxxxxxxx at http://localhost:9090
worker listening on :9090
```


**leave this terminal open as well.**


---


### 6. Test with curl (third terminal)


open **another** terminal. you’ll use it only for testing.


**6a. Check that the API is up**


```powershell
curl http://localhost:8080/health
```


you should see: `OK`.


**6b. Check that the worker is registered**


```powershell
curl http://localhost:8080/ready
```


you should see: `OK` (if you get an error or “no workers”, make sure the worker terminal is still running).


**6c. Submit a job**


```powershell
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d "{\"payload\":\"hello world\"}"
```


you’ll get JSON back with an `"id"` (e.g. `"id":"a1b2c3d4"`). **copy that id** for the next step.


**6d. Get the job status (use your job id)**


replace `YOUR_JOB_ID` with the id you copied (no quotes in the URL):


```powershell
curl http://localhost:8080/jobs/YOUR_JOB_ID
```


run it once right after submitting; you might see `"status":"queued"` or `"status":"running"`. run it again after a second or two; you should see `"status":"completed"` and `"result":"OK:hello world"`.


**6e. See dashboard stats**


```powershell
curl http://localhost:8080/stats
```


you’ll see JSON with `queue_depth`, `workers`, `jobs_total`, `jobs_by_status`, `success_rate_pct`, `uptime_seconds`.


**6f. List all jobs**


```powershell
curl http://localhost:8080/jobs
```


you’ll see your job(s) in the list.


---


### All testing commands (PowerShell and Mac)


Use these once the API and at least one worker are running. On Windows PowerShell, `curl` is an alias for `Invoke-WebRequest` and has different syntax—use the **PowerShell** commands below. On Mac, use the **Mac** commands.


#### PowerShell (Windows)


```powershell
# Health (API up)
Invoke-RestMethod -Uri http://localhost:8080/health


# Ready (at least one worker registered)
Invoke-RestMethod -Uri http://localhost:8080/ready


# Submit a job (returns JSON with id)
Invoke-RestMethod -Uri http://localhost:8080/jobs -Method POST -ContentType "application/json" -Body '{"payload":"hello world"}'


# Get job status (use the id from the response above)
# Option A: run submit and status together
$job = Invoke-RestMethod -Uri http://localhost:8080/jobs -Method POST -ContentType "application/json" -Body '{"payload":"hello world"}'
Invoke-RestMethod -Uri "http://localhost:8080/jobs/$($job.id)"


# Option B: replace JOB_ID with your actual job id
Invoke-RestMethod -Uri http://localhost:8080/jobs/JOB_ID


# Stats (queue depth, workers, jobs by status, success rate, uptime)
Invoke-RestMethod -Uri http://localhost:8080/stats


# List all jobs
Invoke-RestMethod -Uri http://localhost:8080/jobs


# List jobs with pagination (page 2, 10 per page)
Invoke-RestMethod -Uri "http://localhost:8080/jobs?page=2&per_page=10"
```


#### Mac (Terminal / bash)


```bash
# Health (API up)
curl http://localhost:8080/health


# Ready (at least one worker registered)
curl http://localhost:8080/ready


# Submit a job (returns JSON with id)
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"hello world"}'


# Get job status (replace JOB_ID with the id from the submit response)
curl http://localhost:8080/jobs/JOB_ID


# Stats (queue depth, workers, jobs by status, success rate, uptime)
curl http://localhost:8080/stats


# List all jobs
curl http://localhost:8080/jobs


# List jobs with pagination (page 2, 10 per page)
curl "http://localhost:8080/jobs?page=2&per_page=10"
```


---


### 7. Stop everything


- In the **API** terminal: press **Ctrl+C**. you should see something like `shutting down gracefully...` and then it exits.
- In the **worker** terminal: press **Ctrl+C**. the worker will shut down.


you don’t need to do anything in the third terminal (the one you used for curl).


---


### Troubleshooting (Windows)


- **“go is not recognized”** – install Go from https://go.dev/dl/ and then **close and reopen** the terminal.
- **“runner.exe” not found when the worker runs a job** – set `EXECUTION_BINARY` to the **full path**, e.g. `$env:EXECUTION_BINARY = "c:\Users\drjain\Desktop\cloud\runner.exe"`.
- **Port already in use** – make sure you’re not running another program on 8080 or 9090, or stop any previous `api.exe` / `worker.exe`.
- **curl not found** – on Windows 10/11, `curl` is usually available in PowerShell. if not, you can use **Invoke-RestMethod** instead, e.g. `Invoke-RestMethod -Uri http://localhost:8080/health`.


---


## do i need Docker?


**no.** you do **not** need Docker. everything runs with just **Go**:


- **API** = Go program (listens on port 8080).
- **Worker** = Go program (listens on port 9090) that runs each job using a small Go “runner” (no C++ or Docker required).


Docker is only if you want to run API and workers inside containers with one command.


---


## option A – run locally (Go only, no Docker)


### on macOS (MacBook) – quick path


use Terminal. replace `~/Desktop/cloud` with your project path if different.


```bash
cd ~/Desktop/cloud
go mod tidy
go build -o api ./cmd/api
go build -o worker ./cmd/worker
go build -o runner ./cmd/runner
```


**runner on Mac:** the worker needs a “runner” to execute each job. you can either:


- **Use the Go runner** (above): set `export EXECUTION_BINARY="./runner"` when starting the worker.
- **Use the shell script** (no Go runner build): run `chmod +x scripts/runner.sh` once, then set `export EXECUTION_BINARY="$(pwd)/scripts/runner.sh"` (or use the full path, e.g. `~/Desktop/cloud/scripts/runner.sh`) when starting the worker.


**terminal 1 – start API**


```bash
./api
```


leave it running. you should see: `API listening on :8080`.


**terminal 2 – start worker**


```bash
cd ~/Desktop/cloud
export API_URL="http://localhost:8080"
export WORKER_ENDPOINT="http://localhost:9090"
export EXECUTION_BINARY="./runner"
./worker
```


leave it running. you should see: `worker listening on :9090` and `Registered with API as ...`.


**terminal 3 – test**


```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"hello from mac"}'
```


then get the job `id` from the last response and run (replace `JOB_ID` with that id):


```bash
curl http://localhost:8080/jobs/JOB_ID
curl http://localhost:8080/stats
```


to stop: **Ctrl+C** in terminal 1 and terminal 2.


---


### on Windows – step by step


**step 1 – open a terminal in the project folder**


```powershell
cd c:\Users\drjain\Desktop\cloud
```


**what this does:** moves you into the project directory so the next commands run in the right place.


---


### step 2 – download Go dependencies


```powershell
go mod tidy
```


**what this does:** reads `go.mod`, downloads any missing packages (e.g. Docker client for the autoscaler), and updates `go.sum`. you only need to run this once (or when you change dependencies).


---


### step 3 – build the API server


```powershell
go build -o api.exe .\cmd\api
```


**what this does:** compiles the Go code in `cmd/api` (the REST API and scheduler) into a single executable `api.exe` in the current folder.


---


### step 4 – build the worker


```powershell
go build -o worker.exe .\cmd\worker
```


**what this does:** compiles the worker program into `worker.exe`. the worker will register with the API and run jobs.


---


### step 5 – build a “runner” so the worker can execute jobs (no C++ needed)


the worker runs each job by calling a small program (the “runner”). you can use a tiny Go runner instead of the C++ one:


```powershell
go build -o runner.exe .\cmd\runner
```


**what this does:** builds a small program that accepts `--job-id`, `--type`, `--payload` and prints `OK:<payload>`. the worker will call this so jobs complete successfully without needing the C++ binary.


---


### step 6 – start the API (first terminal)


```powershell
.\api.exe
```


**what this does:** starts the API server. it will listen on **http://localhost:8080**, run the scheduler, and wait for workers and job submissions. leave this terminal open. you should see something like: `API listening on :8080`.


---


### step 7 – start the worker (second terminal)


open a **new** terminal, then:


```powershell
cd c:\Users\drjain\Desktop\cloud
$env:API_URL = "http://localhost:8080"
$env:WORKER_ENDPOINT = "http://localhost:9090"
$env:EXECUTION_BINARY = ".\runner.exe"
.\worker.exe
```


**what this does:**


- `$env:API_URL` – tells the worker where the API is (so it can register and report job results).
- `$env:WORKER_ENDPOINT` – tells the API how to reach this worker (localhost:9090).
- `$env:EXECUTION_BINARY` – path to the runner (the Go runner we built; use full path if needed, e.g. `c:\Users\drjain\Desktop\cloud\runner.exe`).
- `.\worker.exe` – starts the worker. it registers with the API and listens on port 9090 for “run job” requests.


leave this terminal open. you should see: `worker listening on :9090` and `Registered with API as ...`.


---


### step 8 – test the API (third terminal or browser)


open another terminal (or use PowerShell in the same window after starting the worker). all of these hit the API on port 8080.


**health check**


```powershell
curl http://localhost:8080/health
```


**what this does:** simple GET request. you should get `OK`. confirms the API is up.


**readiness (should succeed after worker is running)**


```powershell
curl http://localhost:8080/ready
```


**what this does:** returns 200 when at least one worker is registered, 503 otherwise. after the worker is running you should get `OK`.


**submit a job**


```powershell
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d "{\"payload\":\"hello world\"}"
```


**what this does:** sends a POST with a JSON body `{"payload":"hello world"}`. the API creates a job, puts it in the queue, and returns the job (with `id`, `status`, etc.). the scheduler will assign it to your worker, the worker will run `runner.exe` with that payload, and the job will complete.


**get job status (use the job id from the previous response)**


```powershell
curl http://localhost:8080/jobs/JOB_ID_HERE
```


**what this does:** replace `JOB_ID_HERE` with the `id` from the submit response. returns the job’s current status and, when finished, `result` (e.g. `OK:hello world`).


**list all jobs**


```powershell
curl http://localhost:8080/jobs
```


**what this does:** returns all jobs (with pagination). you’ll see the job you submitted and its status.


**dashboard stats**


```powershell
curl http://localhost:8080/stats
```


**what this does:** returns JSON with queue depth, worker count, jobs by status, success rate, uptime. useful to see the system state.


**metrics (Prometheus-style)**


```powershell
curl http://localhost:8080/metrics
```


**what this does:** returns plain-text metrics (queue depth, job counts by status, worker heartbeat age) for monitoring.


---


### step 9 – stop everything


- In the API terminal: press **Ctrl+C**. the API will shut down gracefully.
- In the worker terminal: press **Ctrl+C**. the worker will finish the current job (if any) and then exit.


---


## option B – run with Docker (optional)


you need **Docker Desktop** (or Docker Engine) installed.


### step 1 – go to the project folder


```powershell
cd c:\Users\drjain\Desktop\cloud
```


### step 2 – build and start all services


```powershell
docker compose -f deploy/docker-compose.yaml build
docker compose -f deploy/docker-compose.yaml up -d
```


**what this does:**


- `build` – builds the Docker images for the API and the worker (the worker image includes the C++ runner built inside the container).
- `up -d` – starts the API and two workers in the background (`-d` = detached). the API is on port 8080, workers on their own ports inside the Docker network.


### step 3 – test (same as above)


```powershell
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d "{\"payload\":\"hello from docker\"}"
curl http://localhost:8080/stats
```


use the job `id` from the submit response to check status:


```powershell
curl http://localhost:8080/jobs/JOB_ID_HERE
```


### step 4 – stop and remove containers


```powershell
docker compose -f deploy/docker-compose.yaml down
```


**what this does:** stops and removes the API and worker containers.


---


## quick recap


| step | command | what it does |
|------|--------|---------------|
| 1 | `cd c:\Users\drjain\Desktop\cloud` | go to project folder |
| 2 | `go mod tidy` | download Go dependencies |
| 3 | `go build -o api.exe .\cmd\api` | build the API server |
| 4 | `go build -o worker.exe .\cmd\worker` | build the worker |
| 5 | `go build -o runner.exe .\cmd\runner` | build the Go runner (no C++ needed) |
| 6 | `.\api.exe` (terminal 1) | start the API on port 8080 |
| 7 | set env vars and `.\worker.exe` (terminal 2) | start the worker, register with API, use runner.exe for jobs |
| 8 | `curl -X POST .../jobs -d "{\"payload\":\"hello\"}"` (terminal 3) | submit a job and see it run |
| 9 | `curl .../jobs/<id>` and `curl .../stats` | check job status and dashboard |


you do **not** need Docker to run and test the project; Docker is only for running the same thing in containers.





