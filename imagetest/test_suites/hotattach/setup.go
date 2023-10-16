package hotattach

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "hotattach"

const (
	diskName     = "hotattachmount"
	instanceName = "hotattachvm"

	bootDiskSizeGB  = 10
	mountDiskSizeGB = 30

	// the path to write the file on linux
	linuxMountPath          = "/mnt/disks/hotattach"
	mkfsCmd                 = "mkfs.ext4"
	windowsMountDriveLetter = "F"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if bootDiskSizeGB == mountDiskSizeGB {
		return fmt.Errorf("boot disk and mount disk must be different sizes for disk identification")
	}
	// The extra scope is required to call detachDisk and attachDisk.
	hotattachParams := imagetest.TestVMParams{"extraScopes": "https://www.googleapis.com/auth/cloud-platform"}

	hotattachParams.Disks = []*compute.Disk{{Name: instanceName, Type: imagetest.PdBalanced, SizeGb: bootDiskSizeGB}, {Name: diskName, Type: imagetest.PdBalanced, SizeGb: mountDiskSizeGB}}
	if strings.Contains(t.Image, "arm64") {
		hotattachParams.MachineType = "t2a-standard-8"
	} else {
		hotattachParams.MachineType = "n2-standard-8"
	}
	vm, err := t.CreateTestVMWithParams(hotattachParams)
	if err != nil {
		return err
	}
	vm.RunTests("TestFileHotAttach")
	return nil
}
