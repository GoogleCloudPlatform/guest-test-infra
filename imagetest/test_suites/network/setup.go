package network

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "network"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestDefaultMTU|TestAliasAgentRestart")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.AddAliasIP("10.138.2.0/24")
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestAliasAfterReboot")
	return nil
}
