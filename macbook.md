# How to run and test Kloud on a MacBook (very detailed)


This guide is for **macOS only**. It assumes you are on a Mac (MacBook, iMac, or Mac mini) and walks you through every step from “I don’t have Go” to “I’ve run a job and seen it complete.” You do **not** need Docker.


---


## What you need


- A Mac with macOS (Ventura, Sonoma, or newer is fine).
- **Terminal** (built in: press **Cmd+Space**, type **Terminal**, press Enter).
- The **kloud** project folder on your Mac (e.g. on your Desktop or in Documents).
- Internet connection (for downloading Go and Go modules).


---


## Step 1: Install Go (if you don’t have it)


### 1.1 Check if Go is already installed


1. Open **Terminal** (Cmd+Space → type **Terminal** → Enter).
2. Type this and press Enter:


   ```bash
   go version
   ```


3. If you see something like `go version go1.22.x darwin/arm64` (or `darwin/amd64`), **Go is already installed**. Skip to **Step 2**.
4. If you see `command not found: go` or similar, continue below to install Go.


### 1.2 Download the Go installer for Mac


1. Open your browser (Safari, Chrome, etc.).
2. Go to: **https://go.dev/dl/**
3. On that page, under **Apple macOS**, you’ll see one or more `.pkg` files:
   - **Apple Silicon (M1/M2/M3)** Macs: choose the file that contains **arm64** (e.g. `go1.22.4.darwin-arm64.pkg`).
   - **Intel** Macs: choose the file that contains **amd64** (e.g. `go1.22.4.darwin-amd64.pkg`).
4. Click the `.pkg` link to download it. It will go to your **Downloads** folder.


### 1.3 Run the installer


1. Open **Finder** → **Downloads**.
2. Double‑click the downloaded file (e.g. `go1.22.4.darwin-arm64.pkg`).
3. Follow the installer: **Continue** → **Continue** → **Agree** → **Install** (you may need your Mac password).
4. When it says **The installation was successful**, click **Close**.


### 1.4 Use a new Terminal window


The terminal you had open might not see the new `go` command. Do this:


1. **Quit Terminal** completely (Cmd+Q), or close the Terminal window and open a **new** one (Cmd+N or File → New Window).
2. In the new Terminal, run:


   ```bash
   go version
   ```


3. You should see something like: `go version go1.22.x darwin/arm64` (or `darwin/amd64`).  
   If you still get `command not found`, try **restarting your Mac** once, then open Terminal again and run `go version`.


Once `go version` works, continue to Step 2.


---


## Step 2: Open Terminal and go to the project folder


1. Open **Terminal** (Cmd+Space → **Terminal** → Enter).
2. Go to the folder where the **kloud** project lives. Type one of these (change the path if your project is somewhere else):


   **If kloud is on your Desktop:**


   ```bash
   cd ~/Desktop/kloud
   ```


   **If kloud is in Documents:**


   ```bash
   cd ~/Documents/kloud
   ```


   **If kloud is somewhere else (e.g. Projects):**


   ```bash
   cd ~/Projects/kloud
   ```


3. Check you’re in the right place:


   ```bash
   ls
   ```


   You should see things like `cmd`, `internal`, `go.mod`, `README.md`, `RUN.md`, `scripts`, etc. If you see those, you’re in the **kloud** project folder.


---


## Step 3: Download Go dependencies


In the **same** Terminal window (still in the kloud folder), run:


```bash
go mod tidy
```


**What this does:** Go reads the file `go.mod` in this project, downloads any packages the project needs (e.g. for the API and worker), and updates `go.sum`. You may see it download a few modules; that’s normal. Run this **once** per machine (or after you pull changes that add new dependencies).


If it finishes without errors, continue.


---


## Step 4: Build the three programs


Stay in the **same** Terminal, in the **kloud** folder. Run these **one after the other**:


```bash
go build -o api ./cmd/api
```


**What this does:** Builds the **API server** (the REST API and scheduler). When it finishes, you’ll have a file named **api** (no extension) in the current folder.


```bash
go build -o worker ./cmd/worker
```


**What this does:** Builds the **worker** program that will register with the API and run jobs. You’ll get a file named **worker**.


```bash
go build -o runner ./cmd/runner
```


**What this does:** Builds a small **runner** program that the worker calls to “run” each job (it just prints `OK:<payload>`). You’ll get a file named **runner**. This way you don’t need to build the C++ binary.


**Optional – use the shell script instead of the Go runner:**  
If you prefer **not** to build the Go runner, you can use the included shell script:


```bash
chmod +x scripts/runner.sh
```


**What this does:** Makes `scripts/runner.sh` executable so the worker can run it. You only need to do this **once**. Later, when starting the worker, you’ll set `EXECUTION_BINARY` to the path of `scripts/runner.sh` instead of `./runner` (see Step 7).


Check that the binaries exist:


```bash
ls -l api worker runner
```


You should see the three files. If you see “no such file or directory” for any of them, the corresponding `go build` command failed; scroll up in Terminal to see the error.


**If you get "Permission denied" when you run `./api` (or `./worker` / `./runner`) later:**  
Make the binaries executable with:


```bash
chmod +x api worker runner
```


Then try `./api` again. You only need to do this once after building. OR YOU CAN ALSO COPY THE FILE PATH INTO THE TERMINAL AND CLICK ENTER. 


---


## Step 5: Start the API (first terminal)


You will keep **three** Terminal windows (or tabs): one for the API, one for the worker, one for testing.


1. In **this** Terminal (the one where you ran `go build`), make sure you’re still in the kloud folder:


   ```bash
   cd ~/Desktop/kloud
   ```


   (Use your actual path if different, e.g. `~/Documents/kloud`.)


2. Start the API:


   ```bash
   ./api
   ```


3. You should see a line like:


   ```
   API listening on :8080
   ```


4. **Leave this Terminal open and do not close it.** The API must keep running. Minimize the window if you want, but don’t press Ctrl+C yet.


---


## Step 6: Start the worker (second terminal)


1. Open a **new** Terminal window (Cmd+N) or a new tab (Cmd+T).
2. Go to the **same** project folder again (this new Terminal starts in your home folder, not in kloud):


   ```bash
   cd ~/Desktop/kloud
   ```


   (Again, use your real path if kloud is elsewhere.)


3. Set three environment variables so the worker knows where the API is and how to run jobs. Copy and paste this **whole block** (adjust the path if your project is not on the Desktop):


   **If you built the Go runner (Step 4):**


   ```bash
   export API_URL="http://localhost:8080"
   export WORKER_ENDPOINT="http://localhost:9090"
   export EXECUTION_BINARY="$(pwd)/runner"
   ```


   **If you are using the shell script instead of the Go runner:**


   ```bash
   export API_URL="http://localhost:8080"
   export WORKER_ENDPOINT="http://localhost:9090"
   export EXECUTION_BINARY="$(pwd)/scripts/runner.sh"
   ```


   **What these do:**
   - `API_URL` – tells the worker where the API is so it can register and report job results.
   - `WORKER_ENDPOINT` – tells the API how to reach this worker (localhost:9090).
   - `EXECUTION_BINARY` – full path to the program that runs each job (`./runner` or `scripts/runner.sh`).


4. Start the worker:


   ```bash
   ./worker
   ```


5. You should see something like:


   ```
   Registered with API as worker-xxxxxxxx at http://localhost:9090
   worker listening on :9090
   ```


6. **Leave this Terminal open as well.** Don’t press Ctrl+C here either.


---


## Step 7: Test with curl (third terminal)


1. Open **another** new Terminal window or tab (Cmd+N or Cmd+T).
2. You’ll use this one only for running test commands. You **don’t** need to `cd` into kloud for these; `curl` talks to the API over the network.


### 7.1 Check that the API is up


```bash
curl http://localhost:8080/health
```


**Expected:** The response body should be `OK`.  
If you get “Connection refused” or similar, the API is not running—go back to the first Terminal and run `./api` again.


### 7.2 Check that the worker is registered (readiness)


```bash
curl http://localhost:8080/ready
```


**Expected:** The response body should be `OK`.  
If you get an error or “no workers”, the worker is not registered—make sure the **worker** Terminal is still running `./worker`.


### 7.3 Submit a job


```bash
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"hello from mac"}'
```


**Expected:** You get JSON back with an `"id"` field (e.g. `"id":"a1b2c3d4"`). **Copy or remember that id**—you’ll use it in the next step.


### 7.4 Get the job status


Replace `JOB_ID` with the actual id you got from the previous command (e.g. `a1b2c3d4`). No quotes in the URL:


```bash
curl http://localhost:8080/jobs/JOB_ID
```


Example, if the id was `a1b2c3d4`:


```bash
curl http://localhost:8080/jobs/a1b2c3d4
```


**Expected:** JSON with `"status"` and possibly `"result"`. If you run it right after submitting, you might see `"status":"queued"` or `"status":"running"`. Run it again after a second or two; you should see `"status":"completed"` and `"result":"OK:hello from mac"`.


### 7.5 See dashboard-style stats


```bash
curl http://localhost:8080/stats
```


**Expected:** JSON with `queue_depth`, `workers`, `jobs_total`, `jobs_by_status`, `success_rate_pct`, `uptime_seconds`, etc.


### 7.6 List all jobs


```bash
curl http://localhost:8080/jobs
```


**Expected:** JSON list of jobs (you should see the one you submitted).


### 7.7 List jobs with pagination (optional)


```bash
curl "http://localhost:8080/jobs?page=1&per_page=10"
```


**Expected:** Same as above but with pagination (e.g. `total` and a slice of jobs).


---


## Step 8: Stop everything


When you’re done testing:


1. In the **Terminal where the API is running** (the one showing `API listening on :8080`): press **Ctrl+C**. You should see something like `shutting down gracefully...` and then the program exits.
2. In the **Terminal where the worker is running**: press **Ctrl+C**. The worker will stop.
3. The **third** Terminal (where you ran `curl`) doesn’t need to be stopped; you can just close it or leave it.


You do **not** need to stop anything in a special order; stopping the API first is fine.


---


## Quick reference: all test commands (copy‑paste)


Once the API and worker are running, you can run these in a separate Terminal. Replace `JOB_ID` with a real job id when needed.


```bash
# Health
curl http://localhost:8080/health


# Ready
curl http://localhost:8080/ready


# Submit a job
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"hello world"}'


# Get job status (replace JOB_ID)
curl http://localhost:8080/jobs/JOB_ID


# Stats
curl http://localhost:8080/stats


# List jobs
curl http://localhost:8080/jobs


# List jobs with pagination
curl "http://localhost:8080/jobs?page=2&per_page=10"
```


**Submit and then get status in one go (replace JOB_ID from the first command’s output):**


```bash
curl -X POST http://localhost:8080/jobs -H "Content-Type: application/json" -d '{"payload":"test"}'
# Copy the "id" from the response, then:
curl http://localhost:8080/jobs/JOB_ID
```


---


## Troubleshooting (Mac)


- **`go: command not found`**  
  Go is not installed or not on your PATH. Install Go from **https://go.dev/dl/** (see Step 1), then **quit and reopen Terminal** (or restart the Mac), and run `go version` again.


- **`Permission denied` when running `./runner` or `./api` or `./worker`**  
  The file might not be executable. Run:  
  `chmod +x api worker runner`  
  then try `./api` again.


- **`Permission denied` when the worker runs the shell script runner**  
  Run once:  
  `chmod +x scripts/runner.sh`  
  and make sure `EXECUTION_BINARY` points to the **full path** of `scripts/runner.sh` (e.g. `export EXECUTION_BINARY="$(pwd)/scripts/runner.sh"`).


- **`Address already in use` or `bind: address already in use`**  
  Something else is using port 8080 or 9090. Stop any other copy of `./api` or `./worker`, or quit the app that’s using that port. You can see what’s on a port with:  
  `lsof -i :8080`  
  (replace 8080 with 9090 for the worker).


- **`curl: command not found`**  
  On macOS, `curl` is usually pre-installed. If it’s missing, install Xcode Command Line Tools:  
  `xcode-select --install`  
  then try again.


- **Worker never registers / `/ready` returns error**  
  Make sure you started `./worker` **after** starting `./api`, and that both are still running. Check the worker Terminal for error messages. Ensure `API_URL` is exactly `http://localhost:8080` (no trailing slash).


- **Job stays “queued” or “running” and never completes**  
  The worker might not be able to run the runner. Check that `EXECUTION_BINARY` is the full path to `./runner` or `scripts/runner.sh`, and that `runner.sh` has been made executable with `chmod +x scripts/runner.sh`.


---


## Summary


| Step | What you do |
|------|------------------|
| 1 | Install Go from https://go.dev/dl/ (pick the Mac .pkg for your chip), then open a **new** Terminal and run `go version`. |
| 2 | Open Terminal, `cd` to the kloud folder (e.g. `cd ~/Desktop/kloud`). |
| 3 | Run `go mod tidy`. |
| 4 | Run `go build -o api ./cmd/api`, then `go build -o worker ./cmd/worker`, then `go build -o runner ./cmd/runner`. Optionally use `chmod +x scripts/runner.sh` and use that as the runner instead. |
| 5 | In Terminal 1: `./api` — leave it running. |
| 6 | In Terminal 2: `cd` to kloud, set `API_URL`, `WORKER_ENDPOINT`, `EXECUTION_BINARY`, then `./worker` — leave it running. |
| 7 | In Terminal 3: run `curl` commands to hit `/health`, `/ready`, POST to `/jobs`, GET `/jobs/JOB_ID`, `/stats`, `/jobs`. |
| 8 | Ctrl+C in the API and worker terminals to stop. |


For the same project on **Windows**, see **RUN.md** (Windows section). For a shorter Mac outline, see the “on macOS (MacBook)” section in **RUN.md**.





