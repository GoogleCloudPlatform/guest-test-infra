package storageperf

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	vmName = "vm"
	// iopsErrorMargin allows for a small difference between iops found in the test and the iops value listed in public documentation.
	iopsErrorMargin = 0.97
	// hyperdiskSize in GB is used to determine which partition is the mounted hyperdisk.
	hyperdiskSize = 100
	bootdiskSize  = 50
	mountDiskName = "hyperdisk"
	// TODO: Set up constants for compute.Disk.ProvisionedIOPS int64, and compute.Disk.ProvisionedThrougput int64, then set these fields in appendCreateDisksStep
)

type fioOutput struct {
	jobs []fioJob               `json:"jobs,omitempty"`
	X    map[string]interface{} `json:"-"`
}

type fioJob struct {
	readResult  fioStatistics          `json:"read,omitempty"`
	writeResult fioStatistics          `json:"write,omitempty"`
	X           map[string]interface{} `json:"-"`
}

type fioStatistics struct {
	iops      float64                `json:iops,omitempty"`
	bandwidth float64                `json:bw_mean,omitempty"`
	X         map[string]interface{} `json:"-"`
}

type blockDeviceList struct {
	blockDevices []blockDevice `json:"blockdevices,omitempty"`
}

type blockDevice struct {
	name       string `json:"name,omitempty"`
	size       string `json:"size,omitempty"`
	deviceType string `json:"type,omitempty"`
	// Other fields are not currently used.
	X map[string]interface{} `json:"-"`
}

func getMountDiskPartition(diskExpectedSizeGb int) (string, error) {
	lsblkCmd := "lsblk"
	if !utils.CheckLinuxCmdExists(lsblkCmd) {
		return "", fmt.Errorf("could not find lsblk")
	}
	lsblkout, err := exec.Command(lsblkCmd, "-o", "name,size,type", "--json").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute lsblk cmd with error: %v", err)
	}

	var blockDevices blockDeviceList
	if err := json.Unmarshal(lsblkout, &blockDevices); err != nil {
		return "", fmt.Errorf("failed to unmarshal lsblk output with error: %v", err)
	}

	diskExpectedSizeGbString := strconv.Itoa(diskExpectedSizeGb) + "G"
	for _, blockDev := range blockDevices.blockDevices {
		if strings.ToLower(blockDev.deviceType) == "disk" && blockDev.size == diskExpectedSizeGbString {
			return blockDev.name, nil
		}
	}

	return "", fmt.Errorf("disk block with size not found")
}

// installFio tries to install fio with any of multiple package managers, and returns an error if all the package managers were not found or failed.
func installFio() error {
	var updateCmd, installFioCmd *exec.Cmd
	if utils.CheckLinuxCmdExists("apt") {
		updateCmd = exec.Command("apt", "-y", "update")
		installFioCmd = exec.Command("apt", "install", "-y", "fio")
	} else if utils.CheckLinuxCmdExists("yum") {
		updateCmd = exec.Command("yum", "check-update")
		installFioCmd = exec.Command("yum", "-y", "install", "fio")
	} else if utils.CheckLinuxCmdExists("zypper") {
		updateCmd = exec.Command("zypper", "refresh")
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", "fio")
	} else if utils.CheckLinuxCmdExists("dnf") {
		updateCmd = exec.Command("dnf", "upgrade")
		installFioCmd = exec.Command("dnf", "-y", "install", "fio")
	} else {
		return fmt.Errorf("no package managers to install fio foud")
	}

	if err := updateCmd.Run(); err != nil {
		return fmt.Errorf("update cmd failed with error: %v", err)
	}
	if err := installFioCmd.Run(); err != nil {
		return fmt.Errorf("install fio command failed with error: %v", err)
	}
	return nil
}

func getMountDiskPartitionSymlink() (string, error) {
	mountDiskSymlink := "/dev/disk/by-id/google-" + mountDiskName
	symlinkRealPath, err := filepath.EvalSymlinks(mountDiskSymlink)
	if err != nil {
		return "", fmt.Errorf("symlink could not be resolved with error: %v", err)
	}
	return symlinkRealPath, nil
}
