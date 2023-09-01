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
	commonFIOWriteOptions = "--name=write_iops_test --filesize=10G --numjobs=8 --time_based --runtime=60s --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --rw=randwrite --group_reporting=1 --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
)

func RunFIOWriteWindows() ([]byte, error) {
	testdiskDrive := windowsDriveLetter + ":\\"
	writeIopsFile := "C:\\fio-write-iops.txt"
	//if procStatus, err := utils.RunPowershellCmd("Initialize-Disk -PartitionStyle GPT -Number 1 -PassThru | New-Partition -DriveLetter " + windowsDriveLetter + " -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Perf-Test' -Confirm:$false"); err != nil {
	//	return []byte{}, fmt.Errorf("Initialize-Disk returned with error: %v, %s, %s", err, procStatus.Stdout, procStatus.Stderr)
	//}
	fioWriteOptionsWindows := " -ArgumentList \"" + commonFIOWriteOptions + " --output=" + writeIopsFile + " --ioengine=windowsaio" + " --thread\"" + " -WorkingDirectory " + testdiskDrive + " -wait"
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

func RunFIOWriteLinux() ([]byte, error) {
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

	fioWriteOptionsLinuxSlice := strings.Fields(commonFIOWriteOptions + " --filename=" + symlinkRealPath + " --ioengine=libaio")
	writeIOPSJson, err := exec.Command("fio", fioWriteOptionsLinuxSlice...).CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf("fio command failed with error: %v", err)
	}
	return writeIOPSJson, nil
}

// TestWriteIOPS checks that write IOPS are around the value listed in public docs.
func TestWriteIOPS(t *testing.T) {
	var writeIOPSJson []byte
	var err error
	if runtime.GOOS == "windows" {
		if writeIOPSJson, err = RunFIOWriteWindows(); err != nil {
			t.Fatalf("windows fio write failed with error: %v", err)
		}
	} else {
		if writeIOPSJson, err = RunFIOWriteLinux(); err != nil {
			t.Fatalf("linux fio write failed with error: %v", err)
		}
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(writeIOPSJson, &fioOut); err != nil {
		t.Fatalf("fio output could not be unmarshalled with error: %v", err)
	}

	finalIOPSValue := fioOut.Jobs[0].WriteResult.IOPS
	//TODO: Update this value to be equal to the input IOPS value, once it is implemented in this testing framework. For now, hyperdisk IOPS are the lesser of 100 IOPS per GiB of disk capacity or 350,000, if unspecified.
	expectedHyperdiskIOPS := math.Min(100*hyperdiskSize, 350000)
	if finalIOPSValue < iopsErrorMargin*expectedHyperdiskIOPS {
		t.Fatalf("iops average was too low: expected close to %f, got  %f", expectedHyperdiskIOPS, finalIOPSValue)
	}
	t.Logf("iops test pass with %f iops, expected at least %f", finalIOPSValue, expectedHyperdiskIOPS)
}
