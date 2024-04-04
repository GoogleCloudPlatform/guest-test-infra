package hotattach

import (
	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "hotattach"

const (
	bootDiskSizeGB = 10

	// the path to write the file on linux
	linuxMountPath          = "/mnt/disks/hotattach"
	mkfsCmd                 = "mkfs.ext4"
	windowsMountDriveLetter = "F"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	hotattachInst := &daisy.Instance{}
	hotattachInst.Scopes = append(hotattachInst.Scopes, "https://www.googleapis.com/auth/cloud-platform")

	hotattach, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "reattachPDBalanced", Type: imagetest.PdBalanced, SizeGb: bootDiskSizeGB}, {Name: "hotattachmount", Type: imagetest.PdBalanced, SizeGb: 30}}, hotattachInst)
	if err != nil {
		return err
	}
	hotattach.AddMetadata("hotattach-disk-name", "hotattachmount")
	hotattach.RunTests("TestFileHotAttach")

	if t.Image.Architecture != "ARM64" && utils.HasFeature(t.Image, "GVNIC") {
		lssdMountInst := &daisy.Instance{}
		lssdMountInst.Zone = "us-east4-b"
		lssdMountInst.MachineType = "c3-standard-8-lssd"

		lssdMount, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Zone: "us-east4-b", Name: "remountLSSD", Type: imagetest.PdBalanced, SizeGb: bootDiskSizeGB}}, lssdMountInst)
		if err != nil {
			return err
		}
		// local SSD's don't show up exactly as their device name under /dev/disk/by-id
		if utils.HasFeature(t.Image, "WINDOWS") {
			lssdMount.AddMetadata("hotattach-disk-name", "nvme_card0")
		} else {
			lssdMount.AddMetadata("hotattach-disk-name", "local-nvme-ssd-0")
		}
		lssdMount.RunTests("TestMount")
	}
	return nil
}
