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
	commonFIORandWriteOptions = "--name=write_iops_test --filesize=10G --numjobs=8 --time_based --runtime=60s --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --rw=randwrite --group_reporting=1 --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	commonFIOSeqWriteOptions  = "--name=write_bandwidth_test --filesize=10G --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --numjobs=4 --thread --offset_increment=2G --bs=1M --iodepth=64 --rw=write --iodepth_batch_submit=64  --iodepth_batch_complete_max=64 --output-format=json"
)

func RunFIOWriteWindows(mode string) ([]byte, error) {
	testdiskDrive := windowsDriveLetter + ":\\"
	writeIopsFile := "C:\\fio-write-iops.txt"
	var writeOptions string
	if mode == sequentialMode {
		writeOptions = commonFIOSeqWriteOptions
	} else {
		writeOptions = commonFIORandWriteOptions
	}
	fioWriteOptionsWindows := " -ArgumentList \"" + writeOptions + " --output=" + writeIopsFile + " --ioengine=windowsaio" + " --thread\"" + " -WorkingDirectory " + testdiskDrive + " -wait"
	// fioWindowsLocalPath is defined within storage_perf_utils.go
	if procStatus, err := utils.RunPowershellCmd("Start-Process " + fioWindowsLocalPath + fioWriteOptionsWindows); err != nil {
		return []byte{}, fmt.Errorf("fio.exe returned with error: %v %s %s", err, procStatus.Stdout, procStatus.Stderr)
	}

	writeIopsJsonProcStatus, err := utils.RunPowershellCmd("Get-Content " + writeIopsFile)
	if err != nil {
		return []byte{}, fmt.Errorf("Get-Content of fio output file returned with error: %v %s %s", err, writeIopsJsonProcStatus.Stdout, writeIopsJsonProcStatus.Stderr)
	}
	return []byte(writeIopsJsonProcStatus.Stdout), nil
}

func getLinuxSymlinkWrite() (string, error) {
	symlinkRealPath := ""
	diskPartition, err := utils.GetMountDiskPartition(hyperdiskSize)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		errorString := err.Error()
		symlinkRealPath, err = utils.GetMountDiskPartitionSymlink(mountDiskName)
		if err != nil {
			errorString += err.Error()
			return "", fmt.Errorf("failed to find symlink to mount disk with any method: errors %s", errorString)
		}
	}
	return symlinkRealPath, nil
}

func RunFIOWriteLinux(mode string) ([]byte, error) {
	var writeOptions string
	if mode == sequentialMode {
		writeOptions = commonFIOSeqWriteOptions
	} else {
		writeOptions = commonFIORandWriteOptions
	}
	symlinkRealPath, err := getLinuxSymlinkWrite()
	if err != nil {
		return []byte{}, err
	}
	fioWriteOptionsLinuxSlice := strings.Fields(writeOptions + " --filename=" + symlinkRealPath + " --ioengine=libaio")
	writeIOPSJson, err := exec.Command("fio", fioWriteOptionsLinuxSlice...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf("fio command failed with error: %v", err)
	}
	return writeIOPSJson, nil
}

// TestRandomWriteIOPS checks that random write IOPS are around the value listed in public docs.
func TestRandomWriteIOPS(t *testing.T) {
	var randWriteIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if randWriteIOPSJson, err = RunFIOWriteWindows(randomMode); err != nil {
			t.Fatalf("windows fio rand write failed with error: %v", err)
		}
	} else {
		if randWriteIOPSJson, err = RunFIOWriteLinux(randomMode); err != nil {
			t.Fatalf("linux fio rand write failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(randWriteIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(randWriteIOPSJson), err)
	}

	finalIOPSValue := fioOut.Jobs[0].WriteResult.IOPS
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, hyperdisk IOPS are the lesser of 100 IOPS per GiB of disk capacity or 350,000, if unspecified.
	expectedHyperdiskIOPS := math.Min(100*hyperdiskSize, 350000)
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedHyperdiskIOPS)
}

// TestSequentialWriteIOPS checks that sequential write IOPS are around the value listed in public docs.
func TestSequentialWriteIOPS(t *testing.T) {
	var seqWriteIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if seqWriteIOPSJson, err = RunFIOWriteWindows(sequentialMode); err != nil {
			t.Fatalf("windows fio seq write failed with error: %v", err)
		}
	} else {
		if seqWriteIOPSJson, err = RunFIOWriteLinux(sequentialMode); err != nil {
			t.Fatalf("linux fio seq write failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(seqWriteIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(seqWriteIOPSJson), err)
	}
	var finalIOPSValue float64 = 0.0
	for _, job := range fioOut.Jobs {
		finalIOPSValue += job.WriteResult.IOPS
	}
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, it is not clear what the sequential write iops value should be.
	expectedHyperdiskIOPS := 5.0 * hyperdiskSize
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedHyperdiskIOPS)
}
