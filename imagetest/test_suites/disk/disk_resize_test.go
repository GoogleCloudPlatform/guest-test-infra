package disk

import (
	"flag"
	"fmt"
	"golang.org/x/sys/unix"
	"math"
	"os"
	"testing"
)

const (
	gb = 1024.0 * 1024.0 * 1024.0
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
	diskSize := uint64(stat.Blocks) * uint64(stat.Bsize)
	expectedSize := expectedGb * gb
	maxDiff := float64(expectedSize) * 0.1
	if math.Abs(float64(diskSize)-float64(expectedSize)) > maxDiff {
		return fmt.Errorf("disk size of %d gb not close enough to expected size of %d gb", diskSize/gb, expectedGb)
	}
	return nil
}
