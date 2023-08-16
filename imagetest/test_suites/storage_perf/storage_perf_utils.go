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

// FIOOutput defines the output from the fio command
type FIOOutput struct {
	Jobs []FIOJob               `json:"jobs,omitempty"`
	X    map[string]interface{} `json:"-"`
}

// FIOJob defines one of the jobs listed in the FIO output.
type FIOJob struct {
	ReadResult  FIOStatistics          `json:"read,omitempty"`
	WriteResult FIOStatistics          `json:"write,omitempty"`
	X           map[string]interface{} `json:"-"`
}

// FIOStatistics give information about FIO performance.
type FIOStatistics struct {
	IOPS      float64                `json:iops,omitempty"`
	Bandwidth float64                `json:bw_mean,omitempty"`
	X         map[string]interface{} `json:"-"`
}

// BlockDeviceList gives full information about blockdevices, from the output of lsblk.
type BlockDeviceList struct {
	BlockDevices []BlockDevice `json:"blockdevices,omitempty"`
}

// BlockDevice defines information about a single partition or disk in the output of lsblk.
type BlockDevice struct {
	Name string `json:"name,omitempty"`
	Size string `json:"size,omitempty"`
	Type string `json:"type,omitempty"`
	// Other fields are not currently used.
	X map[string]interface{} `json:"-"`
}

// This method currently only runs the commands for linux. Support for getting the partition on windows will be added in the future.
func getMountDiskPartition(diskExpectedSizeGb int) (string, error) {
	lsblkCmd := "lsblk"
	if !utils.CheckLinuxCmdExists(lsblkCmd) {
		return "", fmt.Errorf("could not find lsblk")
	}
	lsblkout, err := exec.Command(lsblkCmd, "-o", "name,size,type", "--json").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute lsblk cmd with error: %v", err)
	}

	var blockDevices BlockDeviceList
	if err := json.Unmarshal(lsblkout, &blockDevices); err != nil {
		return "", fmt.Errorf("failed to unmarshal lsblk output with error: %v", err)
	}

	diskExpectedSizeGbString := strconv.Itoa(diskExpectedSizeGb) + "G"
	for _, blockDev := range blockDevices.BlockDevices {
		if strings.ToLower(blockDev.Type) == "disk" && blockDev.Size == diskExpectedSizeGbString {
			return blockDev.Name, nil
		}
	}

	return "", fmt.Errorf("disk block with size not found")
}

// installFio tries to install fio with any of multiple package managers, and returns an error if all the package managers were not found or failed.
func installFio() error {
	var installFioCmd *exec.Cmd
	if utils.CheckLinuxCmdExists("apt") {
		// only run update if using apt
		if _, err := exec.Command("apt", "-y", "update").CombinedOutput(); err != nil {
			return fmt.Errorf("apt update failed with error: %v", err)
		}
		installFioCmd = exec.Command("apt", "install", "-y", "fio")
	} else if utils.CheckLinuxCmdExists("yum") {
		installFioCmd = exec.Command("yum", "-y", "install", "fio")
	} else if utils.CheckLinuxCmdExists("zypper") {
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", "fio")
	} else if utils.CheckLinuxCmdExists("dnf") {
		installFioCmd = exec.Command("dnf", "-y", "install", "fio")
	} else {
		return fmt.Errorf("no package managers to install fio foud")
	}

	if _, err := installFioCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install fio command failed with errors: %v", err)
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
