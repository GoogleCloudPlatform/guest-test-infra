//go:build linux && cit
// +build linux,cit

package disk

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	markerFile      = "/boot-marker"
	gb              = 1024.0 * 1024.0 * 1024.0
	defaultDiskSize = 20
)

// TestDiskResize Validate the filesystem is resized on reboot after a disk resize.
func TestDiskResize(t *testing.T) {
	utils.LinuxOnly(t)
	image, err := utils.GetMetadata("image")
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
	if err := verifyDiskSize(resizeDiskSize); err != nil {
		t.Fatal(err)
	}
}

func verifyDiskSize(expectedGb int) error {
	var stat unix.Statfs_t
	if err := unix.Statfs("/", &stat); err != nil {
		return err
	}
	// Total blocks * size per block = total space in bytes
	diskSize := stat.Blocks * uint64(stat.Bsize)
	expectedSize := expectedGb * gb
	maxDiff := float64(expectedSize) * 0.1
	if math.Abs(float64(diskSize)-float64(expectedSize)) > maxDiff {
		return fmt.Errorf("disk size of %d gb not close enough to expected size of %d gb", diskSize/gb, expectedGb)
	}
	return nil
}
