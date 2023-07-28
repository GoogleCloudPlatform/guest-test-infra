package network

import (
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "network"

// Basic instance config for setting up test VMs.
type InstanceConfig struct {
	name string
  ip   string
}

var vm1Config    = InstanceConfig{name: "vm1", ip: "192.168.0.2"}
var vm2Config    = InstanceConfig{name: "vm2", ip: "192.168.0.3"}
var serverConfig = InstanceConfig{name: "server-vm", ip: "192.168.0.4"}
var clientConfig = InstanceConfig{name: "client-vm", ip: "192.168.0.5"}

const (
	serverStartupScript = "startupscripts/netserver_startup.sh"
	clientStartupScript = "startupscripts/netclient_startup.sh"
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

	// VM2 for multiNIC
	vm2, err := t.CreateTestVM(vm2Config.name)
	if err != nil {
		return err
	}
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

	// Create two VMs for GVNIC performance testing.
	serverVM, err := t.CreateTestVM(serverConfig.name)
	if err != nil {
		return err
	}
	if err := serverVM.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := serverVM.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := serverVM.SetPrivateIP(network2, serverConfig.ip); err != nil {
		return err
	}
	if err := serverVM.AddAliasIPRanges("10.14.8.0/24", "secondary-range"); err != nil {
		return err
	}
	serverVM.AddMetadata("enable-guest-attributes", "TRUE")
	serverVM.SetStartupScript(serverStartupScript)
	if err := serverVM.Reboot(); err != nil {
		return err
	}

	clientVM, err := t.CreateTestVM(clientConfig.name)
	if err != nil {
		return err
	}
	if err := clientVM.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := clientVM.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := clientVM.SetPrivateIP(network2, clientConfig.ip); err != nil {
		return err
	}
	if err := clientVM.AddAliasIPRanges("10.14.8.0/24", "secondary-range"); err != nil {
		return err
	}
	clientVM.AddMetadata("enable-guest-attributes", "TRUE")
	clientVM.AddMetadata("iperftarget", serverConfig.ip)
	clientVM.SetStartupScript(clientStartupScript)
	if err := clientVM.Reboot(); err != nil {
		return err
	}

	vm1.RunTests("TestPingVMToVM|TestDHCP|TestDefaultMTU")

	if strings.Contains(t.Image, "debian-10") || strings.Contains(t.Image, "rhel-7-7-sap") || strings.Contains(t.Image, "rhel-8-1-sap") {
		// GVNIC is not supported on some older distros.
		clientVM.RunTests("TestAlias")
	} else {
		clientVM.UseGVNIC()
		serverVM.UseGVNIC()
		clientVM.RunTests("TestAlias|TestGVNIC")
	}
	return nil
}
