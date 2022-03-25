package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/guest-test-infra/container_images/gce-img-resource/in"
)

func fatal(message string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: %s <dest directory>\n", os.Args[0])
	}

	var request in.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		fatal("error reading request from stdin: %s", err)
	}

	response, err := in.Run(os.Args[1], request)
	if err != nil {
		fatal("error getting image: %s", err)
	}

	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		fatal("error writing response to stdout: %s", err)
	}
}
