//go:build cit
// +build cit

package pdbalanceperf

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
	commonFIOSeqReadOptions  = "--name=read_bandwidth_test --filesize=2500G --numjobs=16 --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --thread --offset_increment=100G --bs=1M --iodepth=64 --rw=read --iodepth_batch_submit=64  --iodepth_batch_complete_max=64 --output-format=json"
)

func RunFIOReadWindows(mode string) ([]byte, error) {
	// there is no mounted disk so always assume the C drive
	testdiskDrive := "C:\\"
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
	diskPartition, err := utils.GetMountDiskPartition(bootdiskSizeGB)
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
	expectedRandReadIOPSString, err := utils.GetMetadataAttribute(randReadAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribute %s: err %v", randReadAttribute, err)
	}

	expectedRandReadIOPSString = strings.TrimSpace(expectedRandReadIOPSString)
	var expectedRandReadIOPS float64
	if expectedRandReadIOPS, err := strconv.ParseFloat(expectedRandReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %f was not a float: err %v", expectedRandReadIOPS, err)
	}
	if finalIOPSValue < iopsErrorMargin*expectedRandReadIOPS {
		t.Fatalf("iops average was too low: expected at least %f of target %f, got %f", iopsErrorMargin, expectedRandReadIOPS, finalIOPSValue)
	}

	t.Logf("iops test pass with %f iops, expected at least %f of target %f", finalIOPSValue, iopsErrorMargin, expectedRandReadIOPS)
}

// TestSequentialReadBW checks that sequential read bandwidth values are around the value listed in public docs.
func TestSequentialReadBW(t *testing.T) {
	var seqReadBWJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if seqReadBWJson, err = RunFIOReadWindows(sequentialMode); err != nil {
			t.Fatalf("windows fio seq read failed with error: %v", err)
		}
	} else {
		if seqReadBWJson, err = RunFIOReadLinux(sequentialMode); err != nil {
			t.Fatalf("linux fio seq read failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(seqReadBWJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(seqReadBWJson), err)
	}

	// bytes is listed in bytes per second in the fio output
	finalBandwidthBytesPerSecond := 0
	for _, job := range fioOut.Jobs {
		finalBandwidthBytesPerSecond += job.ReadResult.BandwidthBytes
	}
	var finalBandwidthMBps float64 = float64(finalBandwidthBytesPerSecond) / bytesInMB

	expectedSeqReadBWString, err := utils.GetMetadataAttribute(seqReadAttribute)
	if err != nil {
		t.Fatalf("could not get guest metadata %s: err r%v", seqReadAttribute, err)
	}

	expectedSeqReadBWString = strings.TrimSpace(expectedSeqReadBWString)
	var expectedSeqReadBW float64
	if expectedSeqReadBW, err := strconv.ParseFloat(expectedSeqReadBWString, 64); err != nil {
		t.Fatalf("benchmark bw string %f was not a float: err %v", expectedSeqReadBW, err)
	}
	if finalBandwidthMBps < bwErrorMargin*expectedSeqReadBW {
		t.Fatalf("bw average was too low: expected at least %f of target %f, got %f", bwErrorMargin, expectedSeqReadBW, finalBandwidthMBps)
	}

	t.Logf("bw test pass with %f bw, expected at least %f of target %f", finalBandwidthMBps, bwErrorMargin, expectedSeqReadBW)
}
