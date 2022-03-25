package check

import (
	"context"
	"fmt"
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

// Request is the input of a resource check.
type Request struct {
	Source  gceimgresource.Source  `json:"source"`
	Version gceimgresource.Version `json:"version"`
}

// Response is the output of a resource check.
type Response []gceimgresource.Version

// Run performs a check for image versions.
func Run(request Request) (Response, error) {
	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		return Response{}, err
	}

	call := computeService.Images.List(request.Source.Project)
	if request.Source.Family != "" {
		call = call.Filter(fmt.Sprintf("family = %s", request.Source.Family))
	}

	var is []*compute.Image
	var pt string
	for il, err := call.PageToken(pt).Do(); ; il, err = call.PageToken(pt).Do() {
		if err != nil {
			return Response{}, err
		}
		is = append(is, il.Items...)
		if il.NextPageToken == "" {
			break
		}
		pt = il.NextPageToken
	}

	if request.Version.Name == "" && len(is) > 0 {
		// No version specified, return only the latest image.
		image := is[len(is)-1]

		version, err := mkVersion(image.Name, image.CreationTimestamp)
		if err != nil {
			return Response{}, err
		}

		return Response{version}, nil
	}

	// Use this for correct encoding of empty list.
	response := Response{}

	var start bool
	for _, image := range is {
		if image.Name == request.Version.Name {
			// Start appending from the matching version.
			start = true
		}
		if start {
			version, err := mkVersion(image.Name, image.CreationTimestamp)
			if err != nil {
				return Response{}, err
			}
			response = append(response, version)
		}
	}

	return response, nil
}

func mkVersion(name, timestring string) (gceimgresource.Version, error) {
	creationTime, err := time.Parse(time.RFC3339, timestring)
	if err != nil {
		return gceimgresource.Version{}, err
	}
	return gceimgresource.Version{
		Name:    name,
		Version: fmt.Sprintf("%d", creationTime.Unix()),
	}, nil
}
