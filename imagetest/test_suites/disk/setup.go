package disk

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

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
	if err := vm.ResizeDiskAndReboot(resizeDiskSize); err != nil {
		return err
	}
	vm.RunTests("TestDiskResize")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.RunTests("TestBasicRootFromPD")
	if err = vm2.DetachDisk(); err != nil {
		return err
	}

	vm3, err := t.CreateTestVMWithCustomDisk("vm3", "vm2", "vm2")
	if err != nil {
		return err
	}
	vm3.RunTests("TestBasicRootFromPD")
	return nil
}
