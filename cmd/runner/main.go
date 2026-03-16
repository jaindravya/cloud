// job runner with built-in job types: hash, prime, fetch, sleep, image-resize, compress, email.
// falls back to echo for unknown types so existing clients keep working.
// usage: runner --job-id id --type type --payload payload
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"math"
	"net"
	"net/smtp"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	case "image-resize":
		err = runImageResize(*payload)
	case "compress":
		err = runCompress(*payload)
	case "email":
		err = runEmail(*payload)
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

// --- image-resize -----------------------------------------------------------

type imageResizePayload struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

type imageResizeResult struct {
	OutputPath string `json:"output_path"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Format     string `json:"format"`
}

const maxImageDimension = 8000

func runImageResize(raw string) error {
	var p imageResizePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("invalid image-resize payload: %w", err)
	}
	if p.InputPath == "" || p.OutputPath == "" {
		return fmt.Errorf("image-resize payload requires non-empty \"input_path\" and \"output_path\"")
	}
	if p.Width <= 0 || p.Height <= 0 {
		return fmt.Errorf("image-resize payload requires width and height > 0")
	}
	if p.Width > maxImageDimension || p.Height > maxImageDimension {
		return fmt.Errorf("image dimensions exceed max %dx%d", maxImageDimension, maxImageDimension)
	}

	inPath, err := resolveUnderDataRoot(p.InputPath, true)
	if err != nil {
		return fmt.Errorf("invalid input_path: %w", err)
	}
	outPath, err := resolveUnderDataRoot(p.OutputPath, false)
	if err != nil {
		return fmt.Errorf("invalid output_path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	inFile, err := os.Open(inPath)
	if err != nil {
		return fmt.Errorf("failed to open input image: %w", err)
	}
	defer inFile.Close()

	src, _, err := image.Decode(inFile)
	if err != nil {
		return fmt.Errorf("failed to decode input image: %w", err)
	}

	dst := resizeNearest(src, p.Width, p.Height)

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create output image: %w", err)
	}
	defer outFile.Close()

	format, err := encodeByOutputExtension(outFile, outPath, dst)
	if err != nil {
		return err
	}

	out, _ := json.Marshal(imageResizeResult{
		OutputPath: p.OutputPath,
		Width:      p.Width,
		Height:     p.Height,
		Format:     format,
	})
	fmt.Println(string(out))
	return nil
}

func resizeNearest(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + (y*srcH)/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + (x*srcW)/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func encodeByOutputExtension(w io.Writer, outPath string, img image.Image) (string, error) {
	ext := strings.ToLower(filepath.Ext(outPath))
	switch ext {
	case ".png":
		return "png", png.Encode(w, img)
	case ".jpg", ".jpeg":
		return "jpeg", jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
	case ".gif":
		return "gif", gif.Encode(w, img, nil)
	default:
		return "", fmt.Errorf("unsupported output image extension %q (use .png, .jpg/.jpeg, or .gif)", ext)
	}
}

// --- compress ---------------------------------------------------------------

type compressPayload struct {
	InputPaths []string `json:"input_paths"`
	OutputPath string   `json:"output_path"`
	Format     string   `json:"format,omitempty"`
}

type compressResult struct {
	Format     string `json:"format"`
	OutputPath string `json:"output_path"`
	FileCount  int    `json:"file_count"`
	TotalBytes int64  `json:"total_bytes"`
}

type archiveFile struct {
	absPath string
	arcPath string
	size    int64
}

const (
	maxCompressEntries    = 1000
	maxCompressTotalBytes = 512 * 1024 * 1024 // 512mb
)

func runCompress(raw string) error {
	var p compressPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("invalid compress payload: %w", err)
	}
	if len(p.InputPaths) == 0 {
		return fmt.Errorf("compress payload requires non-empty \"input_paths\"")
	}
	if p.OutputPath == "" {
		return fmt.Errorf("compress payload requires non-empty \"output_path\"")
	}

	format := strings.ToLower(strings.TrimSpace(p.Format))
	if format == "" {
		format = "zip"
	}
	if format != "zip" && format != "tar.gz" {
		return fmt.Errorf("unsupported format %q (use \"zip\" or \"tar.gz\")", format)
	}

	outPathAbs, err := resolveUnderDataRoot(p.OutputPath, false)
	if err != nil {
		return fmt.Errorf("invalid output_path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outPathAbs), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	files, totalBytes, err := collectArchiveFiles(p.InputPaths)
	if err != nil {
		return err
	}

	switch format {
	case "zip":
		if err := writeZipArchive(outPathAbs, files); err != nil {
			return err
		}
	case "tar.gz":
		if err := writeTarGzArchive(outPathAbs, files); err != nil {
			return err
		}
	}

	out, _ := json.Marshal(compressResult{
		Format:     format,
		OutputPath: p.OutputPath,
		FileCount:  len(files),
		TotalBytes: totalBytes,
	})
	fmt.Println(string(out))
	return nil
}

func collectArchiveFiles(inputPaths []string) ([]archiveFile, int64, error) {
	dataRootAbs, err := filepath.Abs(getEnv("RUNNER_DATA_ROOT", "./data"))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve data root: %w", err)
	}

	var (
		files      []archiveFile
		totalBytes int64
		seen       = map[string]struct{}{}
	)

	for _, in := range inputPaths {
		abs, err := resolveUnderDataRoot(in, true)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid input_paths entry %q: %w", in, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to stat %q: %w", in, err)
		}

		if !info.IsDir() {
			af, err := buildArchiveFile(abs, dataRootAbs)
			if err != nil {
				return nil, 0, err
			}
			if _, ok := seen[af.arcPath]; ok {
				continue
			}
			seen[af.arcPath] = struct{}{}
			files = append(files, af)
			totalBytes += af.size
			if err := checkArchiveLimits(len(files), totalBytes); err != nil {
				return nil, 0, err
			}
			continue
		}

		walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlinks are not allowed in compress inputs (%s)", path)
			}

			af, err := buildArchiveFile(path, dataRootAbs)
			if err != nil {
				return err
			}
			if _, ok := seen[af.arcPath]; ok {
				return nil
			}
			seen[af.arcPath] = struct{}{}
			files = append(files, af)
			totalBytes += af.size
			return checkArchiveLimits(len(files), totalBytes)
		})
		if walkErr != nil {
			return nil, 0, fmt.Errorf("failed to collect files from %q: %w", in, walkErr)
		}
	}

	if len(files) == 0 {
		return nil, 0, fmt.Errorf("no files found to compress")
	}

	return files, totalBytes, nil
}

func buildArchiveFile(absPath, dataRootAbs string) (archiveFile, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return archiveFile{}, err
	}
	if info.IsDir() {
		return archiveFile{}, errors.New("cannot archive directories directly")
	}
	rel, err := filepath.Rel(dataRootAbs, absPath)
	if err != nil {
		return archiveFile{}, err
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") || rel == ".." {
		return archiveFile{}, fmt.Errorf("resolved archive path escapes data root: %s", rel)
	}
	return archiveFile{
		absPath: absPath,
		arcPath: rel,
		size:    info.Size(),
	}, nil
}

func checkArchiveLimits(count int, totalBytes int64) error {
	if count > maxCompressEntries {
		return fmt.Errorf("too many files to compress (%d > %d)", count, maxCompressEntries)
	}
	if totalBytes > maxCompressTotalBytes {
		return fmt.Errorf("input size exceeds limit (%d > %d bytes)", totalBytes, maxCompressTotalBytes)
	}
	return nil
}

func writeZipArchive(outPath string, files []archiveFile) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create zip output: %w", err)
	}
	defer outFile.Close()

	zw := zip.NewWriter(outFile)
	defer zw.Close()

	for _, f := range files {
		info, err := os.Stat(f.absPath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", f.absPath, err)
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create zip header for %s: %w", f.absPath, err)
		}
		hdr.Name = f.arcPath
		hdr.Method = zip.Deflate

		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return fmt.Errorf("failed to create zip entry %s: %w", f.arcPath, err)
		}

		src, err := os.Open(f.absPath)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %w", f.absPath, err)
		}
		_, copyErr := io.Copy(w, src)
		closeErr := src.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to write zip entry %s: %w", f.arcPath, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close source file %s: %w", f.absPath, closeErr)
		}
	}
	return nil
}

func writeTarGzArchive(outPath string, files []archiveFile) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create tar.gz output: %w", err)
	}
	defer outFile.Close()

	gzw := gzip.NewWriter(outFile)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for _, f := range files {
		info, err := os.Stat(f.absPath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", f.absPath, err)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header for %s: %w", f.absPath, err)
		}
		hdr.Name = f.arcPath
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("failed to write tar header %s: %w", f.arcPath, err)
		}

		src, err := os.Open(f.absPath)
		if err != nil {
			return fmt.Errorf("failed to open source file %s: %w", f.absPath, err)
		}
		_, copyErr := io.Copy(tw, src)
		closeErr := src.Close()
		if copyErr != nil {
			return fmt.Errorf("failed to write tar entry %s: %w", f.arcPath, copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close source file %s: %w", f.absPath, closeErr)
		}
	}
	return nil
}

// --- email ------------------------------------------------------------------

type emailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text,omitempty"`
	HTML    string `json:"html,omitempty"`
}

type emailResult struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Status  string `json:"status"`
}

const (
	maxEmailSubjectLen = 998
	maxEmailBodyLen    = 1024 * 1024 // 1mb total for text + html
	defaultSMTPTimeout = 30
)

func runEmail(raw string) error {
	var p emailPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("invalid email payload: %w", err)
	}
	p.To = strings.TrimSpace(p.To)
	p.Subject = strings.TrimSpace(p.Subject)
	if p.To == "" {
		return fmt.Errorf("email payload requires non-empty \"to\"")
	}
	if !strings.Contains(p.To, "@") {
		return fmt.Errorf("email \"to\" must contain @")
	}
	if p.Subject == "" {
		return fmt.Errorf("email payload requires non-empty \"subject\"")
	}
	if p.Text == "" && p.HTML == "" {
		return fmt.Errorf("email payload requires at least one of \"text\" or \"html\"")
	}
	if len(p.Subject) > maxEmailSubjectLen {
		return fmt.Errorf("subject length %d exceeds max %d", len(p.Subject), maxEmailSubjectLen)
	}
	totalBody := len(p.Text) + len(p.HTML)
	if totalBody > maxEmailBodyLen {
		return fmt.Errorf("total body size %d exceeds max %d", totalBody, maxEmailBodyLen)
	}

	host := getEnv("SMTP_HOST", "")
	portStr := getEnv("SMTP_PORT", "")
	user := getEnv("SMTP_USER", "")
	pass := getEnv("SMTP_PASS", "")
	from := getEnv("SMTP_FROM", "")
	mode := strings.ToLower(strings.TrimSpace(getEnv("SMTP_MODE", "starttls")))

	if host == "" || portStr == "" {
		return fmt.Errorf("email job requires SMTP_HOST and SMTP_PORT to be set")
	}
	if user == "" || pass == "" {
		return fmt.Errorf("email job requires SMTP_USER and SMTP_PASS to be set")
	}
	if from == "" {
		from = user
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("SMTP_PORT must be a valid port number (1-65535)")
	}

	timeoutSec := defaultSMTPTimeout
	if s := getEnv("SMTP_TIMEOUT_SEC", ""); s != "" {
		if t, err := strconv.Atoi(s); err == nil && t > 0 {
			timeoutSec = t
		}
	}

	if err := sendSMTP(smtpConfig{
		host:    host,
		port:    port,
		user:    user,
		pass:    pass,
		from:    from,
		mode:    mode,
		timeout: time.Duration(timeoutSec) * time.Second,
		to:      p.To,
		subject: p.Subject,
		text:    p.Text,
		html:    p.HTML,
	}); err != nil {
		return err
	}

	out, _ := json.Marshal(emailResult{
		To:      p.To,
		Subject: p.Subject,
		Status:  "sent",
	})
	fmt.Println(string(out))
	return nil
}

type smtpConfig struct {
	host    string
	port    int
	user    string
	pass    string
	from    string
	mode    string
	timeout time.Duration
	to      string
	subject string
	text    string
	html    string
}

func sendSMTP(c smtpConfig) error {
	addr := net.JoinHostPort(c.host, strconv.Itoa(c.port))

	var body bytes.Buffer
	body.WriteString("Subject: ")
	body.WriteString(c.subject)
	body.WriteString("\r\n")
	body.WriteString("MIME-Version: 1.0\r\n")
	if c.html != "" && c.text != "" {
		boundary := "cloud-boundary"
		body.WriteString("Content-Type: multipart/alternative; boundary=" + boundary + "\r\n\r\n")
		body.WriteString("--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n")
		body.WriteString(c.text)
		body.WriteString("\r\n--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\n\r\n")
		body.WriteString(c.html)
		body.WriteString("\r\n--" + boundary + "--\r\n")
	} else if c.html != "" {
		body.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
		body.WriteString(c.html)
	} else {
		body.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
		body.WriteString(c.text)
	}

	auth := smtp.PlainAuth("", c.user, c.pass, c.host)

	switch c.mode {
	case "smtps":
		// connect with tls first (port 465)
		tlsConfig := &tls.Config{ServerName: c.host}
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: c.timeout}, "tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, c.host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer client.Close()
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := client.Mail(c.from); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		if err := client.Rcpt(c.to); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := body.WriteTo(w); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("smtp data close: %w", err)
		}
		return nil
	default:
		// starttls (e.g. port 587)
		conn, err := net.DialTimeout("tcp", addr, c.timeout)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		defer conn.Close()
		client, err := smtp.NewClient(conn, c.host)
		if err != nil {
			return fmt.Errorf("smtp new client: %w", err)
		}
		defer client.Close()
		if err := client.StartTLS(&tls.Config{ServerName: c.host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := client.Mail(c.from); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		if err := client.Rcpt(c.to); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := body.WriteTo(w); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("smtp data close: %w", err)
		}
		return nil
	}
}

// --- path safety ------------------------------------------------------------

func resolveUnderDataRoot(relPath string, mustExist bool) (string, error) {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return "", fmt.Errorf("empty path")
	}
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	clean := filepath.Clean(relPath)
	if clean == "." || clean == "" {
		return "", fmt.Errorf("invalid path")
	}
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal is not allowed")
	}

	baseAbs, err := filepath.Abs(getEnv("RUNNER_DATA_ROOT", "./data"))
	if err != nil {
		return "", fmt.Errorf("failed to resolve data root: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, clean))
	if err != nil {
		return "", fmt.Errorf("failed to resolve target path: %w", err)
	}

	relToBase, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("failed to validate target path: %w", err)
	}
	if relToBase == ".." || strings.HasPrefix(relToBase, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes data root")
	}

	if mustExist {
		if _, err := os.Stat(targetAbs); err != nil {
			return "", fmt.Errorf("path does not exist: %w", err)
		}
	}

	return targetAbs, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
