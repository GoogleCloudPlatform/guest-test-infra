package network

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "network"

const (
	vm1Name = "vm1"
	vm2Name = "vm2"
	vm1IP   = "192.168.0.2"
	vm2IP   = "192.168.0.3"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	network1, err := t.CreateNetwork("network-1", false)
	if err != nil {
		return err
	}
	subnetwork1, err := network1.CreateSubnetwork("subnetwork-1", "10.128.0.0/20")
	if err != nil {
		return err
	}
	subnetwork1.AddSecondaryRange("secondary-range", "10.14.0.0/16")
	if err := network1.CreateFirewallRule("allow-icmp-net1", "icmp", nil, []string{"10.128.0.0/20"}); err != nil {
		return err
	}

	network2, err := t.CreateNetwork("network-2", false)
	if err != nil {
		return err
	}
	subnetwork2, err := network2.CreateSubnetwork("subnetwork-2", "192.168.0.0/16")
	if err != nil {
		return err
	}
	if err := network2.CreateFirewallRule("allow-icmp-net2", "icmp", nil, []string{"192.168.0.0/16"}); err != nil {
		return err
	}

	vm1, err := t.CreateTestVM(vm1Name)
	if err != nil {
		return err
	}
	if err := vm1.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := vm1.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := vm1.SetPrivateIP(network2, vm1IP); err != nil {
		return err
	}

	vm2, err := t.CreateTestVM(vm2Name)
	if err != nil {
		return err
	}
	if err := vm2.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := vm2.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := vm2.SetPrivateIP(network2, vm2IP); err != nil {
		return err
	}
	if err := vm2.AddAliasIPRanges("10.14.8.0/24", "secondary-range"); err != nil {
		return err
	}
	if err := vm2.Reboot(); err != nil {
		return err
	}

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.ChangeNicTypeToGVNIC()
	if err := vm3.Reboot(); err != nil {
		return err
	}

	vm1.RunTests("TestPingVMToVM|TestDHCP|TestDefaultMTU")
	vm2.RunTests("TestAlias")
	vm3.RunTests("TestGVNIC")
	return nil
}
