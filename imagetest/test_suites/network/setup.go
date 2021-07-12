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
	vm2              = "vm2"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestDefaultMTU")

	network, err := t.CreateNetwork(networkName, false)
	if err != nil {
		return err
	}
	subNetwork, err := network.CreateSubNetwork(subnetworkName, primaryIPRange)
	if err != nil {
		return err
	}
	subNetwork.AddSecondaryRange(rangeName, secondaryIPRange)

	vm2, err := t.CreateTestVM(vm2)
	if err != nil {
		return err
	}
	vm2.SetCustomNetwork(networkName, subnetworkName)

	if err := vm2.AddAliasIPRanges(aliasIPRange, rangeName); err != nil {
		return err
	}
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestAliasAfterOnBoot|TestAliasAfterReboot|TestAliasAgentRestart")
	return nil
}
