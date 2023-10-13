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

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
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

func waitAttachDiskComplete(ctx context.Context, attachedDiskResource *computepb.AttachedDisk, projectNumber, instanceNameString, instanceZone string) error {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create rest client: err %v", err)
	}
	defer c.Close()

	req := &computepb.AttachDiskInstanceRequest{
		AttachedDiskResource: attachedDiskResource,
		Project:              projectNumber,
		Instance:             instanceNameString,
		Zone:                 instanceZone,
	}
	op, err := c.AttachDisk(ctx, req)
	if err != nil {
		return fmt.Errorf("attach disk failed: err %v", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("attach disk wait failed: err %v", err)
	}
	return nil
}

func waitDetachDiskComplete(ctx context.Context, deviceName, projectNumber, instanceNameString, instanceZone string) error {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create rest client: err %v", err)
	}
	defer c.Close()

	req := &computepb.DetachDiskInstanceRequest{
		DeviceName: deviceName,
		Project:    projectNumber,
		Instance:   instanceNameString,
		Zone:       instanceZone,
	}
	op, err := c.DetachDisk(ctx, req)
	if err != nil {
		return fmt.Errorf("detach disk failed: err %v", err)
	}

	err = op.Wait(ctx)
	if err != nil {
		return fmt.Errorf("detach disk wait failed: err %v", err)
	}
	return nil
}

func waitGetMountDisk(ctx context.Context, projectNumber, instanceNameString, instanceZone string) (*computepb.AttachedDisk, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest client: err %v", err)
	}
	defer c.Close()

	req := &computepb.GetInstanceRequest{
		Instance: instanceNameString,
		Project:  projectNumber,
		Zone:     instanceZone,
	}
	computepbInstance, err := c.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("instances get call failed with error %v", err)
	}
	// return the mounted disk
	attachedDisks := computepbInstance.Disks
	if len(attachedDisks) < 2 {
		return nil, fmt.Errorf("failed to find second disk on instance: num disks %d", len(attachedDisks))
	}
	return attachedDisks[1], nil
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
	instName, err := exec.Command("hostname").Output()
	if err != nil {
		t.Fatalf("failed to get hostname: %v %v", instName, err)
	}

	projectNumber, instanceZone, err := getProjectAndZone()
	if err != nil {
		t.Fatalf("failed to get metadata for project or zone: %v", err)
	}
	instNameString, _, _ := strings.Cut(strings.TrimSpace(string(instName)), ".")
	ctx := context.Background()
	mountDiskResource, err := waitGetMountDisk(ctx, projectNumber, instNameString, instanceZone)
	if err != nil {
		t.Fatalf("get mount disk fail: %v", err)
	}

	diskDeviceName := mountDiskResource.DeviceName
	if err = waitDetachDiskComplete(ctx, *diskDeviceName, projectNumber, instNameString, instanceZone); err != nil {
		t.Fatalf("detach disk fail: %v", err)
	}

	if err = waitAttachDiskComplete(ctx, mountDiskResource, projectNumber, instNameString, instanceZone); err != nil {
		t.Fatalf("detach disk fail: %v", err)
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
