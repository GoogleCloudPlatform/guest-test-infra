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
	commonFIORandReadOptions = "--name=read_iops_test --filesize=2500G --numjobs=1 --time_based --runtime=1m --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --rw=randread --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	commonFIOSeqReadOptions  = "--name=read_bandwidth_test --filesize=2500G --numjobs=1 --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --thread --offset_increment=500G --bs=1M --iodepth=64 --rw=read --iodepth_batch_submit=64  --iodepth_batch_complete_max=64 --output-format=json"
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
		return "", fmt.Errorf("failed to find symlink: %v", err)
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
	expectedRandReadIOPSString, err := utils.GetMetadata(randReadAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribute %s: err %v", randReadAttribute, err)
	}

	expectedRandReadIOPSString = strings.TrimSpace(expectedRandReadIOPSString)
	var expectedRandReadIOPS float64
	if expectedRandReadIOPS, err := strconv.ParseFloat(expectedRandReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %f was not a float: err %v", expectedRandReadIOPS, err)
	}
	if finalIOPSValue < iopsErrorMargin*expectedRandReadIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedRandReadIOPS, finalIOPSValue)
	}

	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedRandReadIOPS)
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

	finalIOPSValue := fioOut.Jobs[0].ReadResult.IOPS
	expectedSeqReadIOPSString, err := utils.GetMetadata(seqReadAttribute)
	if err != nil {
		t.Fatalf("could not get guest metadata %s: err r%v", seqReadAttribute, err)
	}

	expectedSeqReadIOPSString = strings.TrimSpace(expectedSeqReadIOPSString)
	var expectedSeqReadIOPS float64
	if expectedSeqReadIOPS, err := strconv.ParseFloat(expectedSeqReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %f was not a float: err %v", expectedSeqReadIOPS, err)
	}
	if finalIOPSValue < iopsErrorMargin*expectedSeqReadIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedSeqReadIOPS, finalIOPSValue)
	}

	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedSeqReadIOPS)
}
