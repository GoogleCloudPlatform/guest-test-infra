//go:build cit
// +build cit

package hotattach

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

func getProjectAndZone() (string, string, error) {
	var projectZoneUrl string
	if runtime.GOOS == "windows" {
		procStatus, err := utils.RunPowershellCmd("Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri \"http://metadata.google.internal/computeMetadata/v1/instance/zone\"")
		if err != nil {
			return "", "", fmt.Errorf("failed to get project or zone on windows: %v", err)
		}
		projectZoneUrl = strings.TrimSpace(procStatus.Stdout)
	} else {
		projectZoneBytes, err := exec.Command("curl", "http://metadata.google.internal/computeMetadata/v1/instance/zone", "-H", "Metadata-Flavor: Google").Output()
		projectZoneUrl = strings.TrimSpace(string(projectZoneBytes))
		if err != nil {
			return "", "", fmt.Errorf("failed to get project or zone on linux: %v", err)
		}
	}
	// projectZoneUrl should be in the fomrat projects/$PROJECTNUMBER/zone/$ZONE, and we want to pass in just the $ZONE value to detach the disk.
	projectZoneSlice := strings.Split(string(projectZoneUrl), "/")
	if strings.ToLower(projectZoneSlice[0]) != "projects" || strings.ToLower(projectZoneSlice[2]) != "zones" || len(projectZoneSlice) != 4 {
		return "", "", fmt.Errorf("returned string for vm metata was the wrong format: got %s", projectZoneUrl)
	}

	// return format is (projectNumber, instanceZone, nil)
	return projectZoneSlice[1], projectZoneSlice[3], nil
}

func getLinuxMountPath(mountDiskSizeGB int, mountDiskName string) (string, error) {
	symlinkRealPath := ""
	diskPartition, err := utils.GetMountDiskPartition(mountDiskSizeGB)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		errorString := err.Error()
		symlinkRealPath, err = utils.GetMountDiskPartitionSymlink(mountDiskName)
		if err != nil {
			errorString += err.Error()
			return "", fmt.Errorf("failed to find symlink to mount disk with any method: errors %s", errorString)
		}
	}
	return symlinkRealPath, nil
}

func mountLinuxDiskToPath(mountDiskDir string, isReattach bool) error {
	if err := os.MkdirAll(mountDiskDir, 0777); err != nil {
		return fmt.Errorf("could not make mount disk dir %s: error %v", mountDiskDir, err)
	}
	// see constants defined in setup.go
	mountDiskPath, err := getLinuxMountPath(mountDiskSizeGB, diskName)
	if err != nil {
		return err
	}
	if !utils.CheckLinuxCmdExists(mkfsCmd) {
		return fmt.Errorf("could not format mount disk: %s cmd not found", mkfsCmd)
	}
	if !isReattach {
		mkfsFullCmd := exec.Command(mkfsCmd, "-m", "0", "-E", "lazy_itable_init=0,lazy_journal_init=0,discard", "-F", mountDiskPath)
		if stdout, err := mkfsFullCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs cmd failed to complete: %v %v", stdout, err)
		}
	}

	mountCmd := exec.Command("mount", "-o", "discard,defaults", mountDiskPath, mountDiskDir)

	if err := mountCmd.Run(); err != nil {
		return fmt.Errorf("failed to mount disk: %v", err)
	}

	return nil
}

func unmountLinuxDisk() error {
	// see constants defined in setup.go
	mountDiskPath, err := getLinuxMountPath(mountDiskSizeGB, diskName)
	if err != nil {
		return fmt.Errorf("failed to find unmount path: %v", err)
	}
	umountCmd := exec.Command("umount", "-l", mountDiskPath)
	if stdout, err := umountCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run unmount command: %v %v", stdout, err)
	}
	return nil
}

// TestFileHotAttach is a test which checks that a file on a disk is usable, even after the disk was detached and reattached.
func TestFileHotAttach(t *testing.T) {
	fileName := "hotattach.txt"
	fileContents := "cold Attach"
	fileContentsBytes := []byte(fileContents)
	var fileFullPath string
	if runtime.GOOS == "windows" {
		procStatus, err := utils.RunPowershellCmd("Initialize-Disk -PartitionStyle GPT -Number 1 -PassThru | New-Partition -DriveLetter " + windowsMountDriveLetter + " -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Attach-Test' -Confirm:$false")
		if err != nil {
			t.Fatalf("failed to initialize disk on windows: errors %v, %s, %s", err, procStatus.Stdout, procStatus.Stderr)
		}
		fileFullPath = windowsMountDriveLetter + ":\\" + fileName
	} else {
		if err := mountLinuxDiskToPath(linuxMountPath, false); err != nil {
			t.Fatalf("failed to mount linux disk to linuxmountpath %s: error %v", linuxMountPath, err)
		}
		fileFullPath = linuxMountPath + "/" + fileName
	}
	f, err := os.Create(fileFullPath)
	if err != nil {
		t.Fatalf("failed to create file at path %s: error %v", fileFullPath, err)
	}
	_, err = f.WriteString(fileContents)
	if err != nil {
		t.Fatalf("failed to write to file: %v", err)
	}
	// run unmount steps if linux
	if runtime.GOOS != "windows" {
		if err = unmountLinuxDisk(); err != nil {
			t.Fatalf("unmount failed on linux: %v", err)
		}
	}
	ctx := context.Background()
	service, err := compute.NewService(ctx)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	instancesService := compute.NewInstancesService(service)
	instName, err := exec.Command("hostname").Output()
	if err != nil {
		t.Fatalf("failed to get hostname: %v %v", instName, err)
	}

	projectNumber, instanceZone, err := getProjectAndZone()
	if err != nil {
		t.Fatalf("failed to get metadata for project or zone: %v", err)
	}
	instNameString := strings.TrimSpace(string(instName))
	// the instanceGetCall retrieves the attached disk, which is used to reattach the disk.
	instancesGetCall := instancesService.Get(projectNumber, instanceZone, instNameString)
	instancesGetCall = instancesGetCall.Context(ctx)
	testVMInstance, err := instancesGetCall.Do()
	if err != nil {
		t.Fatalf("instances get call failed with error: %v", err)
	}
	if len(testVMInstance.Disks) < 2 {
		t.Fatalf("failed to find second disk on instance: num disks %d", len(testVMInstance.Disks))
	}
	attachedDisk := testVMInstance.Disks[1]

	instancesDetachCall := instancesService.DetachDisk(projectNumber, instanceZone, instNameString, diskName)
	instancesDetachCall = instancesDetachCall.Context(ctx)
	_, err = instancesDetachCall.Do()
	if err != nil {
		t.Fatalf("detach failed with error: %v", err)
	}

	instancesAttachCall := instancesService.AttachDisk(projectNumber, instanceZone, instNameString, attachedDisk)
	instancesAttachCall = instancesAttachCall.Context(ctx)
	_, err = instancesAttachCall.Do()
	if err != nil {
		t.Fatalf("attach failed with error: %v", err)
	}
	// mount again, then read from the file
	if runtime.GOOS == "windows" {
		t.Log("windows disk was successfully reattached")
	} else {
		if err := mountLinuxDiskToPath(linuxMountPath, true); err != nil {
			t.Fatalf("failed to mount linux disk to path %s on reattach: error %v", linuxMountPath, err)
		}
	}
	hotAttachFile, err := os.Open(fileFullPath)
	if err != nil {
		t.Fatalf("file after hot attach reopen could not be opened at path %s: error A%v", fileFullPath, err)
	}

	fileLength, err := hotAttachFile.Read(fileContentsBytes)
	if fileLength == 0 {
		t.Fatalf("hot attach file was empty after reattach")
	}
	if err != nil {
		t.Fatalf("reading file after reattach failed with error: %v", err)
	}

	t.Logf("hot attach success")
}
