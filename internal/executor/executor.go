package executor


import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
    "strings"
    "time"
)


// runner invokes the c++ execution binary for each job
type Runner struct {
    BinaryPath string
    Timeout    time.Duration
}


// new runner creates a runner that uses the given binary path
func NewRunner(binaryPath string) *Runner {
    return &Runner{
        BinaryPath: binaryPath,
        Timeout:    5 * time.Minute,
    }
}


// result holds the outcome of a job execution
type Result struct {
    Success bool
    Output  string
    Error   string
}


// run executes the job via the c++ binary. arguments are passed as --job-id, --type, --payload.
// if timeout_sec > 0 it overrides the default runner timeout.
func (r *Runner) Run(jobID, jobType, payload string, timeoutSec int) (*Result, error) {
    timeout := r.Timeout
    if timeoutSec > 0 {
        timeout = time.Duration(timeoutSec) * time.Second
    }
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, r.BinaryPath,
        "--job-id", jobID,
        "--type", jobType,
        "--payload", payload,
    )
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    err := cmd.Run()
    outStr := strings.TrimSpace(stdout.String())
    errStr := strings.TrimSpace(stderr.String())
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return &Result{Success: false, Error: "execution timeout"}, nil
        }
        msg := err.Error()
        if errStr != "" {
            msg = errStr
        }
        return &Result{Success: false, Output: outStr, Error: msg}, nil
    }
    // exit code 0 = success
    success := cmd.ProcessState.ExitCode() == 0
    if !success {
        return &Result{Success: false, Output: outStr, Error: fmt.Sprintf("exit code %d", cmd.ProcessState.ExitCode())}, nil
    }
    return &Result{Success: true, Output: outStr}, nil
}





