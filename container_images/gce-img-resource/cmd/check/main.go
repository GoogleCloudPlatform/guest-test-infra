package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/guest-test-infra/container_images/gce-img-resource/check"
)

func fatal(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func main() {
	var request check.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fatal("error reading request from stdin: %s", err)
	}

	response, err := check.Run(request)
	if err != nil {
		fatal("error getting images: %s", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fatal("error writing response to stdout: %s", err)
	}
}
