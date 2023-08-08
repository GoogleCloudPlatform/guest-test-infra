//go:build cit
// +build cit

package storageperf

import (
	"path/filepath"
	"os"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	mkfsCmd = "mkfs.ext4"
	testreadOutputDir = "/mnt/disks/mount_dir"
)

// TestIOPSPrint is a placeholder test which prints out info about iops
func TestIOPSPrint(t *testing.T) {
	mountDiskSymlink := "/dev/disk/by-id/google-" + mountDiskName
	symlinkRealPath, err := filepath.EvalSymlinks(mountDiskSymlink)
	if err != nil {
		t.Fatalf("symlink could not be resolved: %v", err)
	}

	if !utils.CheckLinuxCmdExists(mkfsCmd) {
		t.Fatalf("could not format mount disk: %s cmd not found", mkfsCmd)
	}
	mkfsFullCmd := exec.Command(mkfsCmd, "-m", "0", "-E", "lazy_itable_init=0,lazy_journal_init=0,discard", symlinkRealPath)
	if err := mkfsFullCmd.Run(); err != nil {
		t.Fatalf("mkfs cmd failed to complete: %v", err)
	}

	if err :=  os.MkdirAll(testreadOutputDir, 0777); err != nil {
		t.Fatalf("could not make test read output dir: %v", err)
	}

	mountCmd := exec.Command("mount", "-o", "discard,defaults", symlinkRealPath, testreadOutputDir)

	if err := mountCmd.Run(); err != nil {
		t.Fatalf("failed to mount disk: %v", err)
	}
	// os.ReadDir /dev/disk/by-id/google-$mountDiskName
	// filepath.EvalSymLinks on output
	// mkfs.ext4 -m 0 -E lazy_itable_init=0,lazy_journal_init=0,discard symlinkoutput
	// os mkdir
	// mount with command
	// possibly change mod  with os.Chmod
	t.Logf("empty iops print test")
}
