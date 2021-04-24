package imageboot

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "image_boot"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	if err := vm.Reboot(); err != nil {
		t.Skip("reboot workflow failed")
	}
	vm.RunTests("TestGuestBoot|TestGuestReboot")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}

	vm2.EnableSecureBoot()
	vm2.RunTests("TestGuestSecureBoot")
	return nil
}
