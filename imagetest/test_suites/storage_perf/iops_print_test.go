//go:build cit
// +build cit

package storageperf

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	mkfsCmd           = "mkfs.ext4"
	testreadOutputDir = "/mnt/disks/mount_dir"
)

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

func installFio() error {
	var installFioCmd *exec.Cmd
	if utils.CheckLinuxCmdExists("apt") {
		installFioCmd = exec.Command("apt", "install", "-y", "fio")
	} else if utils.CheckLinuxCmdExists("yum") {
		installFioCmd = exec.Command("yum", "-y", "install", "fio")
	} else if utils.CheckLinuxCmdExists("zypper") {
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", "fio")
	} else if utils.CheckLinuxCmdExists("dnf") {
		installFioCmd = exec.Command("dnf", "-y", "install", "fio")
	} else {
		return fmt.Errorf("could not find package manager to install fio")
	}

	if err := installFioCmd.Run(); err != nil {
		return fmt.Errorf("install fio cmd %s failed with error: %v", installFioCmd.Path, err)
	}

	return nil
}

// TestIOPSPrint is a placeholder test which prints out info about iops
func TestIOPSPrint(t *testing.T) {
	//mountDiskSymlink := "/dev/disk/by-id/google-" + mountDiskName
	//symlinkRealPath, err := filepath.EvalSymlinks(mountDiskSymlink)
	//if err != nil {
	//	t.Fatalf("symlink could not be resolved: %v", err)
	//}
	diskPartition, err := getMountDiskPartition(HyperdiskSize)
	if err != nil {
		t.Fatalf("did not find mount disk partition: %v", err)
	}
	symlinkRealPath := "/dev/" + diskPartition
	if !utils.CheckLinuxCmdExists(mkfsCmd) {
		t.Fatalf("could not format mount disk: %s cmd not found", mkfsCmd)
	}
	mkfsFullCmd := exec.Command(mkfsCmd, "-m", "0", "-E", "lazy_itable_init=0,lazy_journal_init=0,discard", symlinkRealPath)
	if err := mkfsFullCmd.Run(); err != nil {
		t.Fatalf("mkfs cmd failed to complete: %v", err)
	}

	if err := os.MkdirAll(testreadOutputDir, 0777); err != nil {
		t.Fatalf("could not make test read output dir: %v", err)
	}

	mountCmd := exec.Command("mount", "-o", "discard,defaults", symlinkRealPath, testreadOutputDir)

	if err := mountCmd.Run(); err != nil {
		t.Fatalf("failed to mount disk: %v", err)
	}

	if err := installFio(); err != nil {
		t.Fatal(err)
	}
	t.Logf("empty iops print test")
}
