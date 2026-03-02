// job runner with built-in job types: hash, prime, fetch, sleep.
// falls back to echo for unknown types so existing clients keep working.
// usage: runner --job-id id --type type --payload payload
package main

import (
    "crypto/sha256"
    "encoding/json"
	"flag"
	"fmt"
    "io"
    "math"
    "net"
    "net/http"
    "net/url"
	"os"
    "strings"
    "time"
)

func main() {
	jobID := flag.String("job-id", "", "job id")
	jobType := flag.String("type", "", "job type")
    payload := flag.String("payload", "", "payload (json or plain string)")
	flag.Parse()
    _ = jobID

	if *payload == "" {
		fmt.Fprintln(os.Stderr, "missing --payload")
		os.Exit(1)
	}

    var err error
    switch *jobType {
    case "hash":
         err = runHash(*payload)
    case "prime":
         err = runPrime(*payload)
    case "fetch":
         err = runFetch(*payload)
    case "sleep":
         err = runSleep(*payload)
    default:
         fmt.Println("OK:" + *payload)
    }
    if err != nil {
         fmt.Fprintln(os.Stderr, err)
         os.Exit(1)
    }
}

// --- hash -----------------------------------------------------------------

type hashPayload struct {
    Input string `json:"input"`
}

func runHash(raw string) error {
    var p hashPayload
    if err := json.Unmarshal([]byte(raw), &p); err != nil {
         return fmt.Errorf("invalid hash payload: %w", err)
    }
    if p.Input == "" {
         return fmt.Errorf("hash payload requires non-empty \"input\" field")
    }
    h := sha256.Sum256([]byte(p.Input))
    fmt.Printf("%x\n", h)
    return nil
}

// --- prime ----------------------------------------------------------------

type primePayload struct {
    N int `json:"n"`
}

const maxPrimeN = 100_000_000

func runPrime(raw string) error {
    var p primePayload
    if err := json.Unmarshal([]byte(raw), &p); err != nil {
         return fmt.Errorf("invalid prime payload: %w", err)
    }
    if p.N < 2 {
         fmt.Println("primes_up_to=0 count=0 elapsed=0ms")
         return nil
    }
    if p.N > maxPrimeN {
         return fmt.Errorf("n=%d exceeds maximum allowed value of %d", p.N, maxPrimeN)
    }

    start := time.Now()
    count := sieveCount(p.N)
    elapsed := time.Since(start)

    fmt.Printf("primes_up_to=%d count=%d elapsed=%s\n", p.N, count, elapsed.Round(time.Millisecond))
    return nil
}

// sieve of eratosthenes; returns count of primes <= n
func sieveCount(n int) int {
    if n < 2 {
         return 0
    }
    composite := make([]bool, n+1)
    limit := int(math.Sqrt(float64(n)))
    for i := 2; i <= limit; i++ {
         if !composite[i] {
              for j := i * i; j <= n; j += i {
                   composite[j] = true
              }
         }
    }
    count := 0
    for i := 2; i <= n; i++ {
         if !composite[i] {
              count++
         }
    }
    return count
}

// --- fetch ----------------------------------------------------------------

type fetchPayload struct {
    URL    string `json:"url"`
    Method string `json:"method,omitempty"`
}

type fetchResult struct {
    Status        int    `json:"status"`
    ContentLength int    `json:"content_length"`
    Body          string `json:"body"`
}

const (
    maxFetchBodyBytes = 4096
    fetchTimeout      = 30 * time.Second
)

func runFetch(raw string) error {
    var p fetchPayload
    if err := json.Unmarshal([]byte(raw), &p); err != nil {
         return fmt.Errorf("invalid fetch payload: %w", err)
    }
    if p.URL == "" {
         return fmt.Errorf("fetch payload requires non-empty \"url\" field")
    }
    method := strings.ToUpper(p.Method)
    if method == "" {
         method = http.MethodGet
    }

    if err := validateFetchURL(p.URL); err != nil {
         return err
    }

    client := &http.Client{Timeout: fetchTimeout}
    req, err := http.NewRequest(method, p.URL, nil)
    if err != nil {
         return fmt.Errorf("bad request: %w", err)
    }
    req.Header.Set("User-Agent", "cloud-runner/1.0")

    resp, err := client.Do(req)
    if err != nil {
         return fmt.Errorf("fetch failed: %w", err)
    }
    defer resp.Body.Close()

    limited := io.LimitReader(resp.Body, maxFetchBodyBytes+1)
    body, _ := io.ReadAll(limited)
    truncated := len(body) > maxFetchBodyBytes
    if truncated {
         body = body[:maxFetchBodyBytes]
    }

    result := fetchResult{
         Status:        resp.StatusCode,
         ContentLength: int(resp.ContentLength),
         Body:          string(body),
    }
    out, _ := json.Marshal(result)
    fmt.Println(string(out))
    return nil
}

// validateFetchURL blocks requests to private/loopback addresses
func validateFetchURL(rawURL string) error {
    parsed, err := url.Parse(rawURL)
    if err != nil {
         return fmt.Errorf("invalid url: %w", err)
    }
    scheme := strings.ToLower(parsed.Scheme)
    if scheme != "http" && scheme != "https" {
         return fmt.Errorf("only http and https schemes are allowed, got %q", scheme)
    }

    host := parsed.Hostname()

    blockedHosts := []string{"localhost", "127.0.0.1", "::1", "0.0.0.0"}
    for _, b := range blockedHosts {
         if strings.EqualFold(host, b) {
              return fmt.Errorf("fetching %s is not allowed (private/loopback address)", host)
         }
    }

    // resolve and check for private ip ranges
    ips, err := net.LookupIP(host)
    if err != nil {
         return fmt.Errorf("dns lookup failed for %q: %w", host, err)
    }
    for _, ip := range ips {
         if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
              return fmt.Errorf("fetching %s (%s) is not allowed (private/internal address)", host, ip)
         }
    }

    return nil
}

// --- sleep ----------------------------------------------------------------

type sleepPayload struct {
    Seconds float64 `json:"seconds"`
}

const maxSleepSeconds = 300

func runSleep(raw string) error {
    var p sleepPayload
    if err := json.Unmarshal([]byte(raw), &p); err != nil {
         return fmt.Errorf("invalid sleep payload: %w", err)
    }
    if p.Seconds <= 0 {
         return fmt.Errorf("sleep seconds must be > 0")
    }
    if p.Seconds > maxSleepSeconds {
         return fmt.Errorf("sleep seconds %.0f exceeds maximum of %d", p.Seconds, maxSleepSeconds)
    }

    dur := time.Duration(p.Seconds * float64(time.Second))
    time.Sleep(dur)
    fmt.Printf("slept for %s\n", dur.Round(time.Millisecond))
    return nil
}
