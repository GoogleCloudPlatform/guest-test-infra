package oslogin

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "oslogin"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.AddMetadata("enable-oslogin", "true")
	vm.RunTests("TestOsLoginEnabled|TestGetentPasswd")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.AddMetadata("enable-oslogin", "false")
	vm2.RunTests("TestOsLoginDisabled")
	return nil
}
