//go:build cit
// +build cit

package hotattach

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"google.golang.org/api/compute/v1"
)

// TestEmptyAttach is a placeholder test for disk attaching.
func TestEmptyAttach(t *testing.T) {
	ctx := context.Background()
	service, err := compute.NewService(ctx)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	instancesService := compute.NewInstancesService(service)
	instName, err := exec.Command("hostname").Output()
	if err != nil {
		t.Fatalf("failed to get hostname: %v %v", instName, err)
	}
	// fullzone looks like projects/$PROJECTNUMBER/zone/$ZONE, and we want to pass in just the $ZONE value to detach the disk.
	projectZoneBytes, err := exec.Command("curl", "http://metadata.google.internal/computeMetadata/v1/instance/zone", "-H", "Metadata-Flavor: Google").Output()
	projectZoneString := strings.TrimSpace(string(projectZoneBytes))
	if err != nil {
		t.Fatalf("failed to get project or zone: %v", err)
	}
	projectZoneSlice := strings.Split(string(projectZoneString), "/")
	if strings.ToLower(projectZoneSlice[0]) != "projects" || strings.ToLower(projectZoneSlice[2]) != "zones" || len(projectZoneSlice) != 4 {
	   t.Fatalf("returned string for vm metata was the wrong format: got %s", projectZoneString)
	}

  // prepare fields for the disk detach call
  projectNumber := projectZoneSlice[1]
  instanceZone := projectZoneSlice[3]
	instancesDetachCall := instancesService.DetachDisk(projectNumber, instanceZone, strings.TrimSpace(string(instName)), diskName)
	instancesDetachCall = instancesDetachCall.Context(ctx)
	_, err = instancesDetachCall.Do()
	if err != nil {
		t.Fatalf("detach failed with error:  %v", err)
	}
}
