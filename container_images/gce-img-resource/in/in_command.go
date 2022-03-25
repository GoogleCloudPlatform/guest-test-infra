package in

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	gceimgresource "github.com/GoogleCloudPlatform/guest-test-infra/container_images/gce-img-resource"
	"google.golang.org/api/compute/v1"
)

/*
{
  "source": {
		"project": "some-project",
		"family": "some-family",
		"regexp": "rhel-8-v([0-9]+).*",
  },
  "version": { "name": "rhel-8-v20220322" }
}
*/

// Request is the input of a get step.
type Request struct {
	Source  gceimgresource.Source  `json:"source"`
	Version gceimgresource.Version `json:"version"`
}

// Response is the output of a get step.
type Response struct {
	Version  gceimgresource.Version `json:"version"`
	Metadata []Metadata             `json:"metadata,omitempty"`
}

// Metadata are informational fields output by a get step, displayed in the web UI.
type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Run performs a get step, writing image metadata to files in the provided resource dir.
func Run(destinationDir string, request Request) (Response, error) {
	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return Response{}, err
	}

	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		return Response{}, err
	}

	image, err := computeService.Images.Get(request.Source.Project, request.Version.Name).Do()
	if err != nil {
		return Response{}, err
	}

	creationTime, err := time.Parse(time.RFC3339, image.CreationTimestamp)
	if err != nil {
		return Response{}, err
	}
	if err := writeOutput(destinationDir, "creation_timestamp", fmt.Sprintf("%d", creationTime.Unix())); err != nil {
		return Response{}, err
	}
	if err := writeOutput(destinationDir, "name", request.Version.Name); err != nil {
		return Response{}, err
	}
	if err := writeOutput(destinationDir, "url", image.SelfLink); err != nil {
		return Response{}, err
	}
	if err := writeOutput(destinationDir, "version", request.Version.Version); err != nil {
		return Response{}, err
	}

	return Response{
		Version: gceimgresource.Version{
			Name:    request.Version.Name,
			Version: request.Version.Version,
		},
		Metadata: []Metadata{
			{Name: "creation_timestamp", Value: image.CreationTimestamp},
			{Name: "description", Value: image.Description},
			{Name: "image_id", Value: fmt.Sprintf("%d", image.Id)},
			{Name: "url", Value: image.SelfLink},
		},
	}, nil
}

func writeOutput(destinationDir, filename string, content string) error {
	return ioutil.WriteFile(filepath.Join(destinationDir, filename), []byte(content), 0644)
}
