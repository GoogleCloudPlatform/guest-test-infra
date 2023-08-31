//go:build cit
// +build cit

package hotattach

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

func getProjectAndZone() (string, string, error) {
	var projectZoneString string
	if runtime.GOOS == "windows" {
		procStatus, err := utils.RunPowershellCmd("Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri \"http://metadata.google.internal/computeMetadata/v1/instance/zone\"")
		if err != nil {
			return "", "", fmt.Errorf("failed to get project or zone on windows: %v", err)
		}
		projectZoneString = strings.TrimSpace(procStatus.Stdout)
	} else {
		projectZoneBytes, err := exec.Command("curl", "http://metadata.google.internal/computeMetadata/v1/instance/zone", "-H", "Metadata-Flavor: Google").Output()
		projectZoneString = strings.TrimSpace(string(projectZoneBytes))
		if err != nil {
			return "", "", fmt.Errorf("failed to get project or zone on linux: %v", err)
		}
	}
	// projectZoneString should be in the fomrat projects/$PROJECTNUMBER/zone/$ZONE, and we want to pass in just the $ZONE value to detach the disk.
	projectZoneSlice := strings.Split(string(projectZoneString), "/")
	if strings.ToLower(projectZoneSlice[0]) != "projects" || strings.ToLower(projectZoneSlice[2]) != "zones" || len(projectZoneSlice) != 4 {
		return "", "", fmt.Errorf("returned string for vm metata was the wrong format: got %s", projectZoneString)
	}

	// return format is (projectNumber, instanceZone, nil)
	return projectZoneSlice[1], projectZoneSlice[3], nil
}

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
	projectNumber, instanceZone, err := getProjectAndZone()
	if err != nil {
		t.Fatalf("failed to get metadata for project or zone: %v", err)
	}
	instancesDetachCall := instancesService.DetachDisk(projectNumber, instanceZone, strings.TrimSpace(string(instName)), diskName)
	instancesDetachCall = instancesDetachCall.Context(ctx)
	_, err = instancesDetachCall.Do()
	if err != nil {
		t.Fatalf("detach failed with error:  %v", err)
	}
}
