//go:build cit
// +build cit

package storageperf

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	mkfsCmd           = "mkfs.ext4"
	testreadOutputDir = "/mnt/disks/mount_dir"
)

type FIOOutput struct {
	Jobs []FIOJob               `json:"jobs,omitempty"`
	X    map[string]interface{} `json:"-"`
}

type FIOJob struct {
	ReadResult  FIOStatistics          `json:"read,omitempty"`
	WriteResult FIOStatistics          `json:"write,omitempty"`
	X           map[string]interface{} `json:"-"`
}

type FIOStatistics struct {
	IOPS      float64                `json:iops,omitempty"`
	Bandwidth float64                `json:bw_mean,omitempty"`
	X         map[string]interface{} `json:"-"`
}

type BlockDeviceList struct {
	BlockDevices []BlockDevice `json:"blockdevices,omitempty"`
}

type BlockDevice struct {
	Name string `json:"name,omitempty"`
	Size string `json:"size,omitempty"`
	Type string `json:"type,omitempty"`
	// Other fields are not currently used.
	X map[string]interface{} `json:"-"`
}

func getMountDiskPartition(diskExpectedSizeGb int) (string, error) {
	lsblkCmd := "lsblk"
	if !utils.CheckLinuxCmdExists(lsblkCmd) {
		return "", fmt.Errorf("could not find lsblk")
	}
	lsblkout, err := exec.Command(lsblkCmd, "-o", "name,size,type", "--json").Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute lsblk: %v", err)
	}

	var blockDevices BlockDeviceList
	if err := json.Unmarshal(lsblkout, &blockDevices); err != nil {
		return "", fmt.Errorf("failed to unmarshal lsblk output: %v", err)
	}

	diskExpectedSizeGbString := strconv.Itoa(diskExpectedSizeGb) + "G"
	for _, blockDev := range blockDevices.BlockDevices {
		if strings.ToLower(blockDev.Type) == "disk" && blockDev.Size == diskExpectedSizeGbString {
			return blockDev.Name, nil
		}
	}

	return "", fmt.Errorf("disk block with size not found")
}

func checkRunUpdateAndInstall(updateCmd, installFioCmd *exec.Cmd) bool {
	if err := updateCmd.Run(); err != nil {
		return false
	}
	if err := installFioCmd.Run(); err != nil {
		return false
	}

	return true
}

func installFio() error {
	success := false
	var updateCmd, installFioCmd *exec.Cmd
	if utils.CheckLinuxCmdExists("apt") {
		updateCmd = exec.Command("apt", "-y", "update")
		installFioCmd = exec.Command("apt", "install", "-y", "fio")
		success = checkRunUpdateAndInstall(updateCmd, installFioCmd)
	}
	if !success && utils.CheckLinuxCmdExists("yum") {
		updateCmd = exec.Command("yum", "check-update")
		installFioCmd = exec.Command("yum", "-y", "install", "fio")
		success = checkRunUpdateAndInstall(updateCmd, installFioCmd)
	}
	if !success && utils.CheckLinuxCmdExists("zypper") {
		updateCmd = exec.Command("zypper", "refresh")
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", "fio")
		success = checkRunUpdateAndInstall(updateCmd, installFioCmd)
	}
	if !success && utils.CheckLinuxCmdExists("dnf") {
		updateCmd = exec.Command("dnf", "upgrade")
		installFioCmd = exec.Command("dnf", "-y", "install", "fio")
		success = checkRunUpdateAndInstall(updateCmd, installFioCmd)
	}

	if !success {
		return fmt.Errorf("could not find package manager to install fio")
	}
	return nil
}

func getMountDiskPartitionSymlink() (string, error) {
	mountDiskSymlink := "/dev/disk/by-id/google-" + mountDiskName
	symlinkRealPath, err := filepath.EvalSymlinks(mountDiskSymlink)
	if err != nil {
		return "", fmt.Errorf("symlink could not be resolved: %v", err)
	}
	return symlinkRealPath, nil
}

// TestIOPSPrint is a placeholder test which prints out info about iops
func TestIOPSPrint(t *testing.T) {
	symlinkRealPath := ""
	diskPartition, err := getMountDiskPartition(HyperdiskSize)
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

	if err := installFio(); err != nil {
		t.Fatal(err)
	}

	// Arbitrary file read size, less than the size of hte hyperdisk in GB.
	fileReadSizeString := strconv.Itoa(HyperdiskSize/10) + "G"
	iopsJson, err := exec.Command("fio", "--name=read_iops_test", "--filename="+symlinkRealPath, "--filesize="+fileReadSizeString, "--time_based", "--ramp_time=2s", "--runtime=1m", "--ioengine=libaio", "--direct=1", "--verify=0", "--randrepeat=0", "--bs=4k", "--iodepth=256", "--rw=randread", "--iodepth_batch_submit=256", "--iodepth_batch_complete_max=256", "--output-format=json").Output()
	if err != nil {
		t.Fatalf("fio command failed with %v", err)
	}

	var fioOut FIOOutput
	if err = json.Unmarshal(iopsJson, &fioOut); err != nil {
		t.Fatalf("fio output could not be unmarshalled: %v", err)
	}

	finalIOPSValue := fioOut.Jobs[0].ReadResult.IOPS
	if finalIOPSValue < 0 {
		t.Fatalf("iops was less than 0: %f", finalIOPSValue)
	}
	t.Logf("empty iops print test")
}
