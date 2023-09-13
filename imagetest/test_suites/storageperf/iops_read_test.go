//go:build cit
// +build cit

package storageperf

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	commonFIORandReadOptions = "--name=read_iops_test --filesize=10G --numjobs=8 --time_based --runtime=60s --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --rw=randread --group_reporting=1 --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	commonFIOSeqReadOptions  = "--name=read_bandwidth_test --filesize=10G --numjobs=4 --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --thread --offset_increment=2G --bs=1M --iodepth=64 --rw=read --iodepth_batch_submit=64  --iodepth_batch_complete_max=64 --output-format=json"
)

func RunFIOReadWindows(mode string) ([]byte, error) {
	testdiskDrive := windowsDriveLetter + ":\\"
	readIopsFile := "C:\\fio-read-iops.txt"
	var readOptions string
	if mode == sequentialMode {
		readOptions = commonFIOSeqReadOptions
	} else {
		readOptions = commonFIORandReadOptions
	}
	fioReadOptionsWindows := " -ArgumentList \"" + readOptions + " --output=" + readIopsFile + " --ioengine=windowsaio" + " --thread\"" + " -WorkingDirectory " + testdiskDrive + " -wait"
	// fioWindowsLocalPath is defined within storage_perf_utils.go
	if procStatus, err := utils.RunPowershellCmd("Start-Process " + fioWindowsLocalPath + fioReadOptionsWindows); err != nil {
		return []byte{}, fmt.Errorf("fio.exe returned with error: %v %s %s", err, procStatus.Stdout, procStatus.Stderr)
	}

	readIopsJsonProcStatus, err := utils.RunPowershellCmd("Get-Content " + readIopsFile)
	if err != nil {
		return []byte{}, fmt.Errorf("Get-Content of fio output file returned with error: %v %s %s", err, readIopsJsonProcStatus.Stdout, readIopsJsonProcStatus.Stderr)
	}
	return []byte(readIopsJsonProcStatus.Stdout), nil
}

func getLinuxSymlinkRead() (string, error) {
	symlinkRealPath := ""
	diskPartition, err := utils.GetMountDiskPartition(hyperdiskSize)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		return "", fmt.Errorf("failed to find symlink for disk: %v", err)
		/* errorString := err.Error()
		symlinkRealPath, err = utils.GetMountDiskPartitionSymlink(mountDiskName)
		if err != nil {
			errorString += err.Error()
			return "", fmt.Errorf("failed to find symlink to mount disk with any method: errors %s", errorString)
		}*/
	}
	return symlinkRealPath, nil
}
func RunFIOReadLinux(mode string) ([]byte, error) {
	var readOptions string
	if mode == sequentialMode {
		readOptions = commonFIOSeqReadOptions
	} else {
		readOptions = commonFIORandReadOptions
	}
	symlinkRealPath, err := getLinuxSymlinkRead()
	if err != nil {
		return []byte{}, err
	}
	fioReadOptionsLinuxSlice := strings.Fields(readOptions + " --filename=" + symlinkRealPath + " --ioengine=libaio")
	readIOPSJson, err := exec.Command("fio", fioReadOptionsLinuxSlice...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf("fio command failed with error: %v", err)
	}
	return readIOPSJson, nil
}

// TestRandomReadIOPS checks that random read IOPS are around the value listed in public docs.
func TestRandomReadIOPS(t *testing.T) {
	var randReadIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if randReadIOPSJson, err = RunFIOReadWindows(randomMode); err != nil {
			t.Fatalf("windows fio rand read failed with error: %v", err)
		}
	} else {
		if randReadIOPSJson, err = RunFIOReadLinux(randomMode); err != nil {
			t.Fatalf("linux fio rand read failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(randReadIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(randReadIOPSJson), err)
	}

	finalIOPSValue := fioOut.Jobs[0].ReadResult.IOPS
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, hyperdisk IOPS are the lesser of 100 IOPS per GiB of disk capacity or 350,000, if unspecified.
	expectedHyperdiskIOPS := math.Min(100*hyperdiskSize, 350000)
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedHyperdiskIOPS)
}

// TestSequentialReadIOPS checks that sequential read IOPS are around the value listed in public docs.
func TestSequentialReadIOPS(t *testing.T) {
	var seqReadIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if seqReadIOPSJson, err = RunFIOReadWindows(sequentialMode); err != nil {
			t.Fatalf("windows fio seq read failed with error: %v", err)
		}
	} else {
		if seqReadIOPSJson, err = RunFIOReadLinux(sequentialMode); err != nil {
			t.Fatalf("linux fio seq read failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(seqReadIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(seqReadIOPSJson), err)
	}
	var finalIOPSValue float64 = 0.0
	for _, job := range fioOut.Jobs {
		finalIOPSValue += job.ReadResult.IOPS
	}
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, it is not clear what the sequential read iops value should be.
	expectedHyperdiskIOPS := 5.0 * hyperdiskSize
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedHyperdiskIOPS)
}
