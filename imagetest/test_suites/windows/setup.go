package windows

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "windows"

const (
	ip1 = "192.168.0.2"
	ip2 = "192.168.0.3"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestGooGetInstalled|TestGooGetAvailable|TestSigned|TestRemoveInstall" +
		"|TestPackagesInstalled|TestPackagesAvailable|TestPackagesSigned")
	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.RunTests("TestRepoManagement")
	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.RunTests("TestNetworkDriverLoaded|TestDriversInstalled|TestDriversRemoved")

	network1, err := t.CreateNetwork("network-1", false)
	if err != nil {
		return err
	}
	subnetwork1, err := network1.CreateSubnetwork("subnetwork-1", "192.168.0.0/24")
	if err != nil {
		return err
	}
	if err := network1.CreateFirewallRule("allow-icmp-net1", "icmp", nil, []string{"192.168.0.0/24"}); err != nil {
		return err
	}

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	if err := vm4.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := vm4.SetPrivateIP(network1, ip1); err != nil {
		return err
	}

	vm5, err := t.CreateTestVM("vm5")
	if err != nil {
		return err
	}
	if err := vm5.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := vm5.SetPrivateIP(network1, ip2); err != nil {
		return err
	}

	vm4.ChangeNicTypeToGVNIC()

	if err := vm4.Reboot(); err != nil {
		return err
	}

	vm5.RunTests("TestEmptyTest")
	vm4.RunTests("TestGVNIC")
	return nil
}
