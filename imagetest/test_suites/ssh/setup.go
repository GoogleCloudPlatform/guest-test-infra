package ssh

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "ssh"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm1, err := t.CreateTestVM("vm1")
	if err != nil {
		return err
	}
	vm1.RunTests("TestVm1")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.RunTests("TestVm2")

	return nil
}
