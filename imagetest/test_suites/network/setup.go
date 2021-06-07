package network

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "network"

const (
	primaryIPRange   = "192.168.209.0/24"
	secondaryIPRange = "10.14.0.0/16"
	aliasIPRange     = "10.14.1.0/24"
	networkName      = "test-net"
	subnetworkName   = "test-subnet"
	rangeName        = "secondary-range"
	VM2              = "vm2"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestDefaultMTU")

	if err := t.CreateNetwork(VM2, networkName); err != nil {
		return err
	}
	if err := t.CreateSubNetwork(VM2, networkName, subnetworkName, rangeName, primaryIPRange, secondaryIPRange); err != nil {
		return err
	}
	vm2, err := t.CreateTestVM(VM2)
	if err != nil {
		return err
	}
	if err := vm2.EnableNetwork(networkName, subnetworkName, aliasIPRange, rangeName); err != nil {
		return err
	}
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestAliasAfterReboot|TestAliasAgentRestart")
	return nil
}
