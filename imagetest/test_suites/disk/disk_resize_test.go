//go:build cit
// +build cit

package disk

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	markerFile      = "/var/boot-marker"
	gb              = 1024.0 * 1024.0 * 1024.0
	defaultDiskSize = 20
)

// TestDiskResize Validate the filesystem is resized on reboot after a disk resize.
func TestDiskResize(t *testing.T) {
	// TODO: test disk resizing on windows
	utils.LinuxOnly(t)
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	if strings.Contains(image, "rhel-7-4-sap") {
		t.Skip("disk expansion not supported on RHEL 7.4")
	}

	_, err = os.Stat(markerFile)

	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
	} else if err != nil {
		t.Fatalf("failed to stat marker file: %+v", err)
	}

	// Total blocks * size per block = total space in bytes
	if err := verifyDiskSize(resizeDiskSize, image); err != nil {
		t.Fatal(err)
	}
}

func getDiskSize(image string) (int64, error) {
	diskPath := "/"
	if strings.Contains(image, "cos") {
		diskPath = "/mnt/stateful_partition"
        }

	fstatOut, err := exec.Command("df", "-B1", "--output=size", diskPath).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("df command failed with error %v", err)
	}
	fstatOutString := strings.TrimSpace(string(fstatOut))
	fstatOutLines := strings.Split(fstatOutString, "\n")
	if len(fstatOutLines) != 2 {
		return 0, fmt.Errorf("expected 2 lines from fstat output, got string %s", fstatOutString)
	}

	for _, fstatOutLine := range fstatOutLines {
		if diskSize, err := strconv.ParseInt(strings.TrimSpace(fstatOutLine), 10, 64); err == nil {
			return diskSize, nil
		}
	}
	return 0, fmt.Errorf("could not find disk size in fstat output %s", fstatOutString)
}

func verifyDiskSize(expectedGb int, image string) error {
	diskSize, err := getDiskSize(image)
	if err != nil {
		return fmt.Errorf("could not get disk size: err %v", err)
	}
	expectedSize := expectedGb * gb
	maxDiff := float64(expectedSize) * 0.1
	if math.Abs(float64(diskSize)-float64(expectedSize)) > maxDiff {
		return fmt.Errorf("disk size of %d gb not close enough to expected size of %d gb", diskSize/gb, expectedGb)
	}
	return nil
}
