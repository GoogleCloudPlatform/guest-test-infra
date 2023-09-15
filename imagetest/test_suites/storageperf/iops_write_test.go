//go:build cit
// +build cit

package storageperf

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	commonFIORandWriteOptions = "--name=write_iops_test --filesize=2500G --numjobs=1 --time_based --runtime=1m --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --offset_increment=500G --rw=randwrite --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	commonFIOSeqWriteOptions  = "--name=write_bandwidth_test --filesize=2500G --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --numjobs=1 --thread --offset_increment=500G --bs=1M --iodepth=64 --rw=write --iodepth_batch_submit=64  --iodepth_batch_complete_max=64 --output-format=json"
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
		return "", fmt.Errorf("failed to find symlink: error %v", err)
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
	expectedRandWriteIOPSString, err := utils.GetMetadata(randWriteAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribut %s: err %v", randWriteAttribute, err)
	}

	expectedRandWriteIOPSString = strings.TrimSpace(expectedRandWriteIOPSString)
	var expectedRandWriteIOPS float64
	if expectedRandWriteIOPS, err := strconv.ParseFloat(expectedRandWriteIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %f was not a float: err %v", expectedRandWriteIOPS, err)
	}
	if finalIOPSValue < iopsErrorMargin*expectedRandWriteIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedRandWriteIOPS, finalIOPSValue)
	}

	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedRandWriteIOPS)
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

	finalIOPSValue := fioOut.Jobs[0].WriteResult.IOPS
	expectedSeqWriteIOPSString, err := utils.GetMetadata(seqWriteAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribute %s: err %v", seqWriteAttribute, err)
	}

	expectedSeqWriteIOPSString = strings.TrimSpace(expectedSeqWriteIOPSString)
	var expectedSeqWriteIOPS float64
	if expectedSeqWriteIOPS, err := strconv.ParseFloat(expectedSeqWriteIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %f was not a float: err %v", expectedSeqWriteIOPS, err)
	}
	if finalIOPSValue < iopsErrorMargin*expectedSeqWriteIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedSeqWriteIOPS, finalIOPSValue)
	}

	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedSeqWriteIOPS)
}
