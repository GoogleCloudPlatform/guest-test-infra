package check

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
		"readyOnly": true,
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

	var filter []string

	if request.Source.ReadyOnly {
		filter = append(filter, "(status = READY)")
	}

	if request.Source.Family != "" {
		filter = append(filter, fmt.Sprintf("(family = %s)", request.Source.Family))
	}

	if len(filter) > 0 {
		call = call.Filter(strings.Join(filter, " "))
	}

	var images []*compute.Image
	var token string
	for il, err := call.PageToken(token).Do(); ; il, err = call.PageToken(token).Do() {
		if err != nil {
			return Response{}, err
		}
		images = append(images, il.Items...)
		if il.NextPageToken == "" {
			break
		}
		token = il.NextPageToken
	}

	// "By default, results are returned in alphanumerical order based on the resource name."
	// - https://cloud.google.com/compute/docs/reference/rest/v1/images/list

	// "[the] check script...must print the array of new versions, in chronological order (oldest first)"
	// - https://concourse-ci.org/implementing-resource-types.html

	sort.Slice(images, func(i, j int) bool {
		// image.CreationTimestamp is a string in rfc3339 format.
		itime, _ := time.Parse(time.RFC3339, images[i].CreationTimestamp)
		jtime, _ := time.Parse(time.RFC3339, images[j].CreationTimestamp)

		return itime.Unix() < jtime.Unix()
	})

	// No version specified, return only the latest image.
	if request.Version.Name == "" && len(images) > 0 {
		image := images[len(images)-1]

		version, err := mkVersion(image.Name, image.CreationTimestamp)
		if err != nil {
			return Response{}, err
		}

		return Response{version}, nil
	}

	// Requested version must at least be included in the response.
	response := Response{request.Version}

	var start bool
	for _, image := range images {
		if image.Name == request.Version.Name {
			// Start appending from the image after the matching version, aka 'newer'.
			start = true
			continue
		}
		if image.Deprecated != nil && image.Deprecated.State == "DEPRECATED" {
			continue
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
