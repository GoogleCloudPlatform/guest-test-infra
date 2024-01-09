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
	commonFIORandReadOptions    = "--name=read_iops_test --filesize=" + mountdiskSizeGBString + "G --numjobs=1 --time_based --runtime=1m --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --rw=randread --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	commonFIOSeqReadOptions     = "--name=read_bandwidth_test --filesize=" + mountdiskSizeGBString + "G --numjobs=1 --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --offset_increment=500G --bs=1M --iodepth=64 --rw=read --iodepth_batch_submit=64 --iodepth_batch_complete_max=64 --output-format=json"
	hyperdiskFIORandReadOptions = "--numjobs=8 --size=500G --time_based --runtime=5m --ramp_time=10s --direct=1 --verify=0 --bs=4K --iodepth=256 --rw=randread --iodepth_batch_submit=256 --iodepth_batch_complete_max=256 --group_reporting --output-format=json"
	hyperdiskFIOSeqReadOptions  = "--numjobs=8 --size=500G --time_based --runtime=5m --ramp_time=10s --direct=1 --verify=0 --bs=1M --iodepth=64 --rw=read --iodepth_batch_submit=64 --iodepth_batch_complete_max=64 --offset_increment=20G --group_reporting --output-format=json"
)

func RunFIOReadWindows(mode string) ([]byte, error) {
	readIopsFile := "C:\\fio-read-iops.txt"
	var readOptions string
	if mode == sequentialMode {
		readOptions = commonFIOSeqReadOptions
	} else {
		readOptions = commonFIORandReadOptions
	}
	fioReadOptionsWindows := " -ArgumentList \"" + readOptions + " --output=" + readIopsFile + " --filename=\\\\.\\PhysicalDrive1" + " --ioengine=windowsaio" + " --thread\"" + " -wait"
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
	diskPartition, err := utils.GetMountDiskPartition(mountdiskSizeGB)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		return "", fmt.Errorf("failed to find symlink: %v", err)
	}
	return symlinkRealPath, nil
}

func getLinuxReadOptions(mode string, usingHyperdisk bool) string {
	if mode == sequentialMode && usingHyperdisk {
		return hyperdiskFIOSeqReadOptions
	} else if mode == sequentialMode && !usingHyperdisk {
		return commonFIOSeqReadOptions
	} else if mode == randomMode && usingHyperdisk {
		return hyperdiskFIORandReadOptions
	} else {
		return commonFIORandReadOptions
	}
}

func RunFIOReadLinux(t *testing.T, mode string) ([]byte, error) {
	ctx := utils.Context(t)
	usingHyperdisk := isUsingHyperdisk(ctx)
	readOptions := getLinuxReadOptions(mode, usingHyperdisk)

	symlinkRealPath, err := getLinuxSymlinkRead()
	if err != nil {
		return []byte{}, err
	}

	// ubuntu 16.04 has a different option name due to an old fio version
	image, err := utils.GetMetadata(ctx, "instance", "image")
	if err != nil {
		return []byte{}, fmt.Errorf("couldn't get image from metadata")
	}
	if strings.Contains(image, "ubuntu-pro-1604") {
		readOptions = strings.Replace(readOptions, "iodepth_batch_complete_max", "iodepth_batch_complete", 1)
	}

	if !utils.CheckLinuxCmdExists(fioCmdNameLinux) {
		if err = installFioLinux(); err != nil {
			return []byte{}, fmt.Errorf("linux fio installation failed: err %v", err)
		}
		// fill the disk right after the fio install, ensuring that fill disk is only run once
		if usingHyperdisk {
			err = fillDisk(symlinkRealPath)
			if err != nil {
				return []byte{}, fmt.Errorf("fill disk preliminary step failed: err %v", err)
			}
		}
	}
	readOptions += " --filename=" + symlinkRealPath + " --ioengine=libaio"
	// use the recommended options from the hyperdisk docs at https://cloud.google.com/compute/docs/disks/benchmark-hyperdisk-performance
	// the options --name and --numa_cpu_node must be at the very end of the command to run the jobs correctly on hyperdisk and avoid confusing fio
	if usingHyperdisk {
		hyperdiskAdditionalOptions, err := getHyperdiskAdditionalOptions(symlinkRealPath)
		if err != nil {
			t.Fatalf("failed to get hyperdisk additional options: error %v", err)
		}
		readOptions += hyperdiskAdditionalOptions
	}

	randReadCmd := exec.Command(fioCmdNameLinux, strings.Fields(readOptions)...)
	t.Logf("read command string is %s", randReadCmd.String())
	readIOPSJson, err := randReadCmd.CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf("fio command failed with error: %v %v", readIOPSJson, err)
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
		if randReadIOPSJson, err = RunFIOReadLinux(t, randomMode); err != nil {
			t.Fatalf("linux fio rand read failed with error: %s", err.Error())
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(randReadIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(randReadIOPSJson), err)
	}

	// this is a json.Number object
	finalIOPSValueNumber := fioOut.Jobs[0].ReadResult.IOPS
	var finalIOPSValue float64
	if finalIOPSValue, err = finalIOPSValueNumber.Float64(); err != nil {
		t.Fatalf("iops json number %s was not a float: %v", finalIOPSValueNumber.String(), err)
	}
	finalIOPSValueString := fmt.Sprintf("%f", finalIOPSValue)
	expectedRandReadIOPSString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", randReadAttribute)
	if err != nil {
		t.Fatalf("could not get metadata attribute %s: err %v", randReadAttribute, err)
	}

	expectedRandReadIOPSString = strings.TrimSpace(expectedRandReadIOPSString)
	var expectedRandReadIOPS float64
	if expectedRandReadIOPS, err = strconv.ParseFloat(expectedRandReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %s was not a float: err %v", expectedRandReadIOPSString, err)
	}

	machineName := getVMName(utils.Context(t))
	if finalIOPSValue < iopsErrorMargin*expectedRandReadIOPS {
		t.Fatalf("iops average for vm %s was too low: expected at least %f of target %s, got %s", machineName, iopsErrorMargin, expectedRandReadIOPSString, finalIOPSValueString)
	}

	t.Logf("iops test pass for vm %s with %s iops, expected at least %f of target %s", machineName, finalIOPSValueString, iopsErrorMargin, expectedRandReadIOPSString)
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
		if seqReadIOPSJson, err = RunFIOReadLinux(t, sequentialMode); err != nil {
			t.Fatalf("linux fio seq read failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(seqReadIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output %s could not be unmarshalled with error: %v", string(seqReadIOPSJson), err)
	}

	// bytes is listed in bytes per second in the fio output
	var finalBandwidthBytesPerSecond int64 = 0
	for _, job := range fioOut.Jobs {
		// this is the bandwidth units/sec as a json.Number object
		bandwidthNumber := job.ReadResult.Bandwidth
		var bandwidthInt int64
		if bandwidthInt, err = bandwidthNumber.Int64(); err != nil {
			t.Fatalf("bandwidth units per second %s was not a float: err  %v", bandwidthNumber.String(), err)
		}
		finalBandwidthBytesPerSecond += bandwidthInt * fioBWToBytes
	}

	var finalBandwidthMBps float64 = float64(finalBandwidthBytesPerSecond) / bytesInMB
	finalBandwidthMBpsString := fmt.Sprintf("%f", finalBandwidthMBps)

	expectedSeqReadIOPSString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", seqReadAttribute)
	if err != nil {
		t.Fatalf("could not get guest metadata %s: err r%v", seqReadAttribute, err)
	}

	expectedSeqReadIOPSString = strings.TrimSpace(expectedSeqReadIOPSString)
	var expectedSeqReadIOPS float64
	if expectedSeqReadIOPS, err = strconv.ParseFloat(expectedSeqReadIOPSString, 64); err != nil {
		t.Fatalf("benchmark iops string %s  was not a float: err %v", expectedSeqReadIOPSString, err)
	}

	machineName := getVMName(utils.Context(t))
	if finalBandwidthMBps < iopsErrorMargin*expectedSeqReadIOPS {
		t.Fatalf("iops average was too low for vm %s: expected at least %f of target %s, got %s", machineName, iopsErrorMargin, expectedSeqReadIOPSString, finalBandwidthMBpsString)
	}

	t.Logf("iops test pass for vm %s with %s iops, expected at least %f of target %s", machineName, finalBandwidthMBpsString, iopsErrorMargin, expectedSeqReadIOPSString)
}
