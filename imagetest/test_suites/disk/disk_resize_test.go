package disk

import (
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

const (
	defaultDiskSize = 20
	gb              = 1024.0 * 1024.0
	markerFile      = "/boot-marker"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

// TestDiskResize Validate the filesystem is resized on reboot after a disk resize.
func TestDiskResize(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		if err := verifyDiskSize(defaultDiskSize); err != nil {
			t.Fatal(err)
		}
	}
	// second boot
	if err := verifyDiskSize(resizeDiskSize); err != nil {
		t.Fatal(err)
	}
}

func verifyDiskSize(expectedGb int) error {
	cmd := exec.Command("df", "-k", "/")
	b, err := cmd.Output()
	if err != nil {
		return err
	}
	diskSizeLine := strings.Split(string(b), "\n")[1]
	diskSize, err := strconv.Atoi(strings.Fields(diskSizeLine)[1])
	if err != nil {
		return err
	}
	expectedSize := expectedGb * gb
	maxDiff := float64(expectedSize * 1 / 100)
	if math.Abs(float64(diskSize)-float64(expectedSize)) > maxDiff {
		return fmt.Errorf("disk size of %d not close enough to expected size of %d", diskSize/gb, expectedSize/gb)
	}
	return nil
}
