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
	vm               = "vm"
	vm2              = "vm2"
	vm3              = "vm3"
	vm4              = "vm4"
	sourceIP         = "192.168.0.2"
	targetIP         = "192.168.0.3"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM(vm)
	if err != nil {
		return err
	}
	vm.RunTests("TestDefaultMTU")

	network, err := t.CreateNetwork(networkName, false)
	if err != nil {
		return err
	}
	subnetwork, err := network.CreateSubnetwork(subnetworkName, primaryIPRange)
	if err != nil {
		return err
	}
	subnetwork.AddSecondaryRange(rangeName, secondaryIPRange)

	vm2, err := t.CreateTestVM(vm2)
	if err != nil {
		return err
	}
	if err := vm2.SetCustomNetwork(network, subnetwork, ""); err != nil {
		return err
	}
	vm2.AddAliasIPRanges(aliasIPRange, rangeName)
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestAliasAfterOnBoot|TestAliasAfterReboot|TestAliasAgentRestart")

	// create network
	network1, err := t.CreateNetwork("network-1", true)
	if err != nil {
		return err
	}
	network2, err := t.CreateNetwork("network-2", false)
	if err != nil {
		return err
	}
	// create subnetwork
	subnetwork2, err := network2.CreateSubnetwork("subnetwork-2", "192.168.0.0/16")
	if err != nil {
		return err
	}
	// create firewall
	if err := t.CreateFirewallRule("allow-icmp-net1", "network-1", "icmp", nil); err != nil {
		return err
	}
	if err := t.CreateFirewallRule("allow-ssh-net1", "network-1", "tcp", []string{"22"}); err != nil {
		return err
	}
	if err := t.CreateFirewallRule("allow-icmp-net2", "network-2", "icmp", nil); err != nil {
		return err
	}
	if err := t.CreateFirewallRule("allow-ssh-net2", "network-2", "tcp", []string{"22"}); err != nil {
		return err
	}

	// vm3 and vm4 are for multinic_minimal_network_test
	vm3, err := t.CreateTestVM(vm3)
	if err != nil {
		return err
	}
	vm4, err := t.CreateTestVM(vm4)
	if err != nil {
		return err
	}
	vm3.AddMetadata("block-project-ssh-keys", "true")

	if err := vm3.SetCustomNetwork(network1, nil, ""); err != nil {
		return err
	}
	if err := vm3.SetCustomNetwork(network2, subnetwork2, sourceIP); err != nil {
		return err
	}
	if err := vm4.SetCustomNetwork(network1, nil, ""); err != nil {
		return err
	}
	if err := vm4.SetCustomNetwork(network2, subnetwork2, targetIP); err != nil {
		return err
	}

	vm3.RunTests("TestVMToVM")
	vm4.RunTests("TestEmptyTest")
	return nil
}
