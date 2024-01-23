//go:build cit
// +build cit

package hotattach

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func getWindowsDiskNumber(ctx context.Context) (int, error) {
	diskName, err := utils.GetMetadata(ctx, "instance", "attributes", "hotattach-disk-name")
	if err != nil {
		return 0, err
	}
	intMatch := regexp.MustCompile("[0-9]+")
	o, err := utils.RunPowershellCmd(`(Get-Disk -FriendlyName "` + diskName + `").Number`)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(intMatch.FindString(o.Stdout))
}

func getLinuxMountPath(ctx context.Context) (string, error) {
	diskName, err := utils.GetMetadata(ctx, "instance", "attributes", "hotattach-disk-name")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks("/dev/disk/by-id/google-" + diskName)
}

func mountLinuxDiskToPath(ctx context.Context, mountDiskDir string, isReattach bool) error {
	if err := os.MkdirAll(mountDiskDir, 0777); err != nil {
		return fmt.Errorf("could not make mount disk dir %s: error %v", mountDiskDir, err)
	}
	mountDiskPath, err := getLinuxMountPath(ctx)
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

func unmountLinuxDisk(ctx context.Context) error {
	mountDiskPath, err := getLinuxMountPath(ctx)
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
	ctx := utils.Context(t)
	fileName := "hotattach.txt"
	fileContents := "cold Attach"
	fileContentsBytes := []byte(fileContents)
	var fileFullPath string
	if runtime.GOOS == "windows" {
		diskNum, err := getWindowsDiskNumber(ctx)
		if err != nil {
			diskNum = 1
		}
		procStatus, err := utils.RunPowershellCmd(fmt.Sprintf(`Initialize-Disk -PartitionStyle GPT -Number %d -PassThru | New-Partition -DriveLetter %s -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Attach-Test' -Confirm:$false`, diskNum, windowsMountDriveLetter))
		if err != nil {
			t.Fatalf("failed to initialize disk on windows: errors %v, %s, %s", err, procStatus.Stdout, procStatus.Stderr)
		}
		fileFullPath = windowsMountDriveLetter + ":\\" + fileName
	} else {
		if err := mountLinuxDiskToPath(ctx, linuxMountPath, false); err != nil {
			t.Fatalf("failed to mount linux disk to linuxmountpath %s: error %v", linuxMountPath, err)
		}
		fileFullPath = linuxMountPath + "/" + fileName
	}
	f, err := os.Create(fileFullPath)
	if err != nil {
		f.Close()
		t.Fatalf("failed to create file at path %s: error %v", fileFullPath, err)
	}

	w := bufio.NewWriter(f)
	_, err = w.Write(fileContentsBytes)
	if err != nil {
		f.Close()
		t.Fatalf("failed to write bytes: err %v", err)
	}
	w.Flush()
	f.Sync()
	if err = f.Close(); err != nil {
		t.Fatalf("possible race condition, file operation not completed: error %v", err)
	}
	// run unmount steps if linux
	if runtime.GOOS != "windows" {
		if err = unmountLinuxDisk(ctx); err != nil {
			t.Fatalf("unmount failed on linux: %v", err)
		}
	}
	instName, err := utils.GetInstanceName(ctx)
	if err != nil {
		t.Fatalf("failed to get instance name: error %v", err)
	}
	instName = strings.TrimSpace(instName)
	projectNumber, instanceZone, err := utils.GetProjectZone(ctx)
	if err != nil {
		t.Fatalf("failed to get instance zone or project details: error %v", err)
	}

	mountDiskResource, err := waitGetMountDisk(ctx, projectNumber, instName, instanceZone)
	if err != nil {
		t.Fatalf("get mount disk fail: projectNumber %s, instanceName %s, instanceZone %s, %v", projectNumber, instName, instanceZone, err)
	}

	diskDeviceName := mountDiskResource.DeviceName
	if err = waitDetachDiskComplete(ctx, *diskDeviceName, projectNumber, instName, instanceZone); err != nil {
		t.Fatalf("detach disk fail: %v", err)
	}

	if err = waitAttachDiskComplete(ctx, mountDiskResource, projectNumber, instName, instanceZone); err != nil {
		t.Fatalf("detach disk fail: %v", err)
	}

	// mount again, then read from the file
	if runtime.GOOS == "windows" {
		t.Log("windows disk was successfully reattached")
	} else {
		if err := mountLinuxDiskToPath(ctx, linuxMountPath, true); err != nil {
			t.Fatalf("failed to mount linux disk to path %s on reattach: error %v", linuxMountPath, err)
		}
	}
	hotAttachFile, err := os.Open(fileFullPath)
	if err != nil {
		hotAttachFile.Close()
		t.Fatalf("file after hot attach reopen could not be opened at path %s: error A%v", fileFullPath, err)
	}
	defer hotAttachFile.Close()

	fileLength, err := hotAttachFile.Read(fileContentsBytes)
	if fileLength == 0 {
		t.Fatalf("hot attach file was empty after reattach")
	}
	if err != nil {
		t.Fatalf("reading file after reattach failed with error: %v", err)
	}

	t.Logf("hot attach success")
}

// TestMount tests that a drive can be mounted and written to. Hotattach without the attaching and detaching.
func TestMount(t *testing.T) {
	ctx := utils.Context(t)
	fileName := "hotattach.txt"
	fileContents := "cold Attach"
	fileContentsBytes := []byte(fileContents)
	var fileFullPath string
	if runtime.GOOS == "windows" {
		diskNum, err := getWindowsDiskNumber(ctx)
		if err != nil {
			diskNum = 1
		}
		procStatus, err := utils.RunPowershellCmd(fmt.Sprintf(`Initialize-Disk -PartitionStyle GPT -Number %d -PassThru | New-Partition -DriveLetter %s -UseMaximumSize | Format-Volume -FileSystem NTFS -NewFileSystemLabel 'Attach-Test' -Confirm:$false`, diskNum, windowsMountDriveLetter))
		if err != nil {
			t.Fatalf("failed to initialize disk on windows: errors %v, %s, %s", err, procStatus.Stdout, procStatus.Stderr)
		}
		fileFullPath = windowsMountDriveLetter + ":\\" + fileName
	} else {
		if err := mountLinuxDiskToPath(ctx, linuxMountPath, false); err != nil {
			t.Fatalf("failed to mount linux disk to linuxmountpath %s: error %v", linuxMountPath, err)
		}
		fileFullPath = linuxMountPath + "/" + fileName
	}
	f, err := os.Create(fileFullPath)
	if err != nil {
		f.Close()
		t.Fatalf("failed to create file at path %s: error %v", fileFullPath, err)
	}

	w := bufio.NewWriter(f)
	_, err = w.Write(fileContentsBytes)
	if err != nil {
		f.Close()
		t.Fatalf("failed to write bytes: err %v", err)
	}
	w.Flush()
	f.Sync()

	hotAttachFile, err := os.Open(fileFullPath)
	if err != nil {
		hotAttachFile.Close()
		t.Fatalf("file could not be reopened at path %s: error A%v", fileFullPath, err)
	}
	defer hotAttachFile.Close()

	fileLength, err := hotAttachFile.Read(fileContentsBytes)
	if fileLength == 0 {
		t.Fatalf("file was empty after writing to it")
	}
	if err != nil {
		t.Fatalf("reading file after writing failed with error: %v", err)
	}
}
