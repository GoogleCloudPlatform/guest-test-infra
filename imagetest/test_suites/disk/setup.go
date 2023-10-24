package disk

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "disk"

const (
	vmName         = "vm"
	resizeDiskSize = 200
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM(vmName)
	if err != nil {
		return err
	}
	if utils.HasFeature(t.Image, "WINDOWS") {
		vm.RunTests(TestDiskReadWrite)
		return nil
	} else {
		return vm.ResizeDiskAndReboot(resizeDiskSize)
	}
}
