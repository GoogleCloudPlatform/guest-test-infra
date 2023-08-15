//go:build cit
// +build cit

package storageperf

import (
	"encoding/json"
	"math"
	"os/exec"
	"strconv"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestReadIOPS checks that read IOPS are around the value listed in public docs.
func TestReadIOPS(t *testing.T) {
	symlinkRealPath := ""
	diskPartition, err := getMountDiskPartition(hyperdiskSize)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		errorString := err.Error()
		symlinkRealPath, err = getMountDiskPartitionSymlink()
		if err != nil {
			errorString += err.Error()
			t.Fatalf("failed to find symlink to mount disk with any method: errors %s", errorString)
		}
	}

	if !utils.CheckLinuxCmdExists("fio") {
		if err := installFio(); err != nil {
			t.Fatal(err)
		}
	}

	// Arbitrary file read size, less than the size of hte hyperdisk in GB.
	fileReadSizeString := strconv.Itoa(hyperdiskSize/10) + "G"
	iopsJson, err := exec.Command("fio", "--name=read_iops_test", "--filename="+symlinkRealPath, "--filesize="+fileReadSizeString, "--time_based", "--ramp_time=2s", "--runtime=1m", "--ioengine=libaio", "--direct=1", "--verify=0", "--randrepeat=0", "--bs=4k", "--iodepth=256", "--rw=randread", "--iodepth_batch_submit=256", "--iodepth_batch_complete_max=256", "--output-format=json").CombinedOutput()
	if err != nil {
		t.Fatalf("fio command failed with error: %v", err)
	}

	var fioOut fioOutput
	if err = json.Unmarshal(iopsJson, &fioOut); err != nil {
		t.Fatalf("fio output could not be unmarshalled with error: %v", err)
	}

	finalIOPSValue := fioOut.jobs[0].readResult.iops
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, hyperdisk IOPS are the lesser of 100 IOPS per GiB of disk capacity or 350,000, if unspecified.
	expectedHyperdiskIOPS := math.Min(100*hyperdiskSize, 350000)
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Log("iops test pass")
}
