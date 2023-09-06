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
	var projectZoneUrl string
	if runtime.GOOS == "windows" {
		procStatus, err := utils.RunPowershellCmd("Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri \"http://metadata.google.internal/computeMetadata/v1/instance/zone\"")
		if err != nil {
			return "", "", fmt.Errorf("failed to get project or zone on windows: %v", err)
		}
		projectZoneUrl = strings.TrimSpace(procStatus.Stdout)
	} else {
		projectZoneBytes, err := exec.Command("curl", "http://metadata.google.internal/computeMetadata/v1/instance/zone", "-H", "Metadata-Flavor: Google").Output()
		projectZoneUrl = strings.TrimSpace(string(projectZoneBytes))
		if err != nil {
			return "", "", fmt.Errorf("failed to get project or zone on linux: %v", err)
		}
	}
// projectZoneUrl should be in the fomrat projects/$PROJECTNUMBER/zone/$ZONE, and we want to pass in just the $ZONE value to detach the disk.
	projectZoneSlice := strings.Split(string(projectZoneUrl), "/")
	if strings.ToLower(projectZoneSlice[0]) != "projects" || strings.ToLower(projectZoneSlice[2]) != "zones" || len(projectZoneSlice) != 4 {
		return "", "", fmt.Errorf("returned string for vm metata was the wrong format: got %s", projectZoneUrl)
	}

	// return format is (projectNumber, instanceZone, nil)
	return projectZoneSlice[1], projectZoneSlice[3], nil
}

// TestFileHotAttach is a test which checks that a file on a disk is usable, even after the disk was detached and reattached.
func TestFileHotAttach(t *testing.T) {
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
	instNameString := strings.TrimSpace(string(instName))
	// the instanceGetCall retrieves the attached disk, which is used to reattach the disk.
	instancesGetCall := instancesService.Get(projectNumber, instanceZone, instNameString)
	instancesGetCall = instancesGetCall.Context(ctx)
	testVMInstance, err := instancesGetCall.Do()
	if err != nil {
		t.Fatalf("instances get call failedw ith error: %v", err)
	}
	if len(testVMInstance.Disks) < 2 {
		t.Fatalf("failed to find second disk on instance: num disks %d", len(testVMInstance.Disks))
	}
	attachedDisk := testVMInstance.Disks[1]

	instancesDetachCall := instancesService.DetachDisk(projectNumber, instanceZone, instNameString, diskName)
	instancesDetachCall = instancesDetachCall.Context(ctx)
	_, err = instancesDetachCall.Do()
	if err != nil {
		t.Fatalf("detach failed with error: %v", err)
	}

	instancesAttachCall := instancesService.AttachDisk(projectNumber, instanceZone, instNameString, attachedDisk)
	instancesAttachCall = instancesAttachCall.Context(ctx)
	_, err = instancesAttachCall.Do()
	if err != nil {
		t.Fatalf("attach failed with error: %v", err)
	}
}
