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
	vm3IP            = "192.168.0.2"
	vm4IP            = "192.168.0.3"
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
	if err := vm2.AddCustomNetwork(network, subnetwork); err != nil {
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
	// create firewall rules
	if err := network1.CreateFirewallRule("allow-icmp-net1", "icmp", nil, nil); err != nil {
		return err
	}
	if err := network1.CreateFirewallRule("allow-ssh-net1", "tcp", []string{"22"}, nil); err != nil {
		return err
	}
	if err := network2.CreateFirewallRule("allow-icmp-net2", "icmp", nil, []string{"192.168.0.0/16"}); err != nil {
		return err
	}
	if err := network2.CreateFirewallRule("allow-ssh-net2", "tcp", []string{"22"}, []string{"192.168.0.0/16"}); err != nil {
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

	if err := vm3.AddCustomNetwork(network1, nil); err != nil {
		return err
	}
	if err := vm3.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := vm3.SetPrivateIP(network2, vm3IP); err != nil {
		return err
	}
	if err := vm4.AddCustomNetwork(network1, nil); err != nil {
		return err
	}
	if err := vm4.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := vm4.SetPrivateIP(network2, vm4IP); err != nil {
		return err
	}
	vm3.RunTests("TestPingVMToVM")
	vm4.RunTests("TestEmptyTest")
	return nil
}
