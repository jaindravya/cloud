// minimal job runner for local testing when the c++ binary is not built.
// usage: runner.exe --job-id id --type type --payload payload
// prints ok:<payload> and exits 0.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	jobID := flag.String("job-id", "", "job id")
	jobType := flag.String("type", "", "job type")
	payload := flag.String("payload", "", "payload")
	flag.Parse()
	if *payload == "" {
		fmt.Fprintln(os.Stderr, "missing --payload")
		os.Exit(1)
	}
	_ = jobID
	_ = jobType
	fmt.Println("OK:" + *payload)
	os.Exit(0)
}
