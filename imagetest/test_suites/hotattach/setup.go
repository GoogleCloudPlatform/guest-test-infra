package hotattach

import (
	"fmt"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "hotattach"

const (
	diskName     = "hotattachMount"
	instanceName = "hotattachvm"

	bootDiskSize  = 10
	mountDiskSize = 30

	// the path to write the file on linux
	linuxMountPath          = "/mnt/disks/hotattach"
	mkfsCmd                 = "mkfs.ext4"
	windowsMountDriveLetter = "F"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if bootDiskSize == mountDiskSize {
		return fmt.Errorf("boot disk and mount disk must be different sizes for disk identification")
	}
	// The extra scope is required to call detachDisk and attachDisk.
	hotattachParams := map[string]string{"machineType": "n2-standard-8", "extraScopes": "https://www.googleapis.com/auth/cloud-platform"}
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: instanceName, Type: imagetest.PdBalanced, SizeGb: bootDiskSize}, {Name: diskName, Type: imagetest.PdBalanced, SizeGb: mountDiskSize}}, hotattachParams)
	if err != nil {
		return err
	}
	vm.RunTests("TestFileHotAttach")
	return nil
}
