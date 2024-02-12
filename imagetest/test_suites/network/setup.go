package network

import (
	"strings"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "network"

// InstanceConfig for setting up test VMs.
type InstanceConfig struct {
	name string
	ip   string
}

var vm1Config = InstanceConfig{name: "vm1", ip: "192.168.0.2"}
var vm2Config = InstanceConfig{name: "vm2", ip: "192.168.0.3"}

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

	vm1, err := t.CreateTestVM(vm1Config.name)
	if err != nil {
		return err
	}
	if err := vm1.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := vm1.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := vm1.SetPrivateIP(network2, vm1Config.ip); err != nil {
		return err
	}
	vm1.RunTests("TestPingVMToVM|TestDHCP|TestDefaultMTU")

	var multinictests string
	if !utils.HasFeature(t.Image, "WINDOWS") && !strings.Contains(t.Image.Name, "sles-15") && !strings.Contains(t.Image.Name, "opensuse-leap") && !strings.Contains(t.Image.Name, "ubuntu-1604") {
		multinictests += "TestAlias"
	}
	if utils.HasFeature(t.Image, "GVNIC") {
		if multinictests != "" {
			multinictests += "|"
		}
		multinictests += "TestGVNIC"
	}

	if multinictests != "" {
		// VM2 for multiNIC
		networkRebootInst := &daisy.Instance{}
		networkRebootInst.Metadata = map[string]string{imagetest.ShouldRebootDuringTest: "true"}
		vm2, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: vm2Config.name}}, networkRebootInst)
		if err != nil {
			return err
		}
		vm2.AddMetadata("enable-guest-attributes", "TRUE")
		if err := vm2.AddCustomNetwork(network1, subnetwork1); err != nil {
			return err
		}
		if err := vm2.AddCustomNetwork(network2, subnetwork2); err != nil {
			return err
		}
		if err := vm2.SetPrivateIP(network2, vm2Config.ip); err != nil {
			return err
		}
		if err := vm2.AddAliasIPRanges("10.14.8.0/24", "secondary-range"); err != nil {
			return err
		}
		if err := vm2.Reboot(); err != nil {
			return err
		}
		if utils.HasFeature(t.Image, "GVNIC") {
			vm2.UseGVNIC()
		}

		vm2.RunTests(multinictests)
	}
	return nil
}
