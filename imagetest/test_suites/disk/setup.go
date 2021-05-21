package disk

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "disk"

const resizeDiskSize = 200

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	if err := vm.ResizeDisk(resizeDiskSize); err != nil {
		return err
	}
	if err := vm.Reboot(); err != nil {
		return err
	}
	return nil
}
