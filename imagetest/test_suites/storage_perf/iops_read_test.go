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
	commonFIOReadOptions = "--name=read_iops_test --filesize=10G --numjobs=8 --time_based --runtime=60s --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --rw=randread --group_reporting=1 --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	windowsDriveLetter   = "F"
)

func RunFIOReadWindows() ([]byte, error) {
	testdiskDrive := windowsDriveLetter + ":\\"
	readIopsFile := "C:\\fio-read-iops.txt"
	if procStatus, err := utils.RunPowershellCmd("Initialize-Disk -PartitionStyle GPT -Number 1 -PassThru | New-Partition -DriveLetter " + windowsDriveLetter + " -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Perf-Test' -Confirm:$false"); err != nil {
		return []byte{}, fmt.Errorf("Initialize-Disk returned with error: %v, %s, %s", err, procStatus.Stdout, procStatus.Stderr)
	}
	fioReadOptionsWindows := " -ArgumentList \"" + commonFIOReadOptions + " --output=" + readIopsFile + " --ioengine=windowsaio" + " --thread\"" + " -WorkingDirectory " + testdiskDrive + " -wait"
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

func RunFIOReadLinux() ([]byte, error) {
	symlinkRealPath := ""
	diskPartition, err := getMountDiskPartition(hyperdiskSize)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		errorString := err.Error()
		symlinkRealPath, err = getMountDiskPartitionSymlink()
		if err != nil {
			errorString += err.Error()
			return []byte{}, fmt.Errorf("failed to find symlink to mount disk with any method: errors %s", errorString)
		}
	}

	fioReadOptionsLinuxSlice := strings.Fields(commonFIOReadOptions + " --filename=" + symlinkRealPath + " --ioengine=libaio")
	readIOPSJson, err := exec.Command("fio", fioReadOptionsLinuxSlice...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf("fio command failed with error: %v", err)
	}
	return readIOPSJson, nil
}

// TestReadIOPS checks that read IOPS are around the value listed in public docs.
func TestReadIOPS(t *testing.T) {
	var readIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if readIOPSJson, err = RunFIOReadWindows(); err != nil {
			t.Fatalf("windows fio read failed with error: %v", err)
		}
	} else {
		if readIOPSJson, err = RunFIOReadLinux(); err != nil {
			t.Fatalf("linux fio read failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(readIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output could not be unmarshalled with error: %v", err)
	}

	finalIOPSValue := fioOut.Jobs[0].ReadResult.IOPS
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, hyperdisk IOPS are the lesser of 100 IOPS per GiB of disk capacity or 350,000, if unspecified.
	expectedHyperdiskIOPS := math.Min(100*hyperdiskSize, 350000)
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedHyperdiskIOPS)
}
