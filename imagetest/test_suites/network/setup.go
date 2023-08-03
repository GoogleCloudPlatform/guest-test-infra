package network

import (
	"embed"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
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
var serverConfig = InstanceConfig{name: "server-vm", ip: "192.168.1.4"}
var clientConfig = InstanceConfig{name: "client-vm", ip: "192.168.1.5"}

//go:embed startupscripts/*
var scripts embed.FS

const (
	serverStartupScriptURL = "startupscripts/netserver_startup.sh"
	clientStartupScriptURL = "startupscripts/netclient_startup.sh"
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

	clientServerNetwork, err := t.CreateNetwork("client-server-network", false)
	if err != nil {
		return err
	}
	clientServerSubnetwork, err := clientServerNetwork.CreateSubnetwork("client-server-subnetwork", "192.168.1.0/24")
	if err != nil {
		return err
	}
	clientServerSubnetwork.AddSecondaryRange("client-server-secondary-range", "10.14.0.0/16")
	if err := clientServerNetwork.CreateFirewallRule("allow-icmp-net-client", "icmp", nil, []string{"192.168.1.0/24"}); err != nil {
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

	// Get the startup scripts as strings.
	serverStartup, err := scripts.ReadFile(serverStartupScriptURL)
	if err != nil {
		return err
	}
	clientStartup, err := scripts.ReadFile(clientStartupScriptURL)
	if err != nil {
		return err
	}

	// Create two VMs for GVNIC performance testing.
	serverVM, err := t.CreateTestVM(serverConfig.name)
	if err != nil {
		return err
	}
	if err := serverVM.AddCustomNetwork(clientServerNetwork, clientServerSubnetwork); err != nil {
		return err
	}
	if err := serverVM.SetPrivateIP(clientServerNetwork, serverConfig.ip); err != nil {
		return err
	}
	serverVM.AddMetadata("enable-guest-attributes", "TRUE")
	serverVM.SetStartupScript(string(serverStartup))
	if err := serverVM.Reboot(); err != nil {
		return err
	}

	clientVM, err := t.CreateTestVM(clientConfig.name)
	if err != nil {
		return err
	}
	if err := clientVM.AddCustomNetwork(clientServerNetwork, clientServerSubnetwork); err != nil {
		return err
	}
	if err := clientVM.SetPrivateIP(clientServerNetwork, clientConfig.ip); err != nil {
		return err
	}
	if err := clientVM.AddAliasIPRanges("10.14.8.0/24", "client-server-secondary-range"); err != nil {
		return err
	}
	clientVM.AddMetadata("enable-guest-attributes", "TRUE")
	clientVM.AddMetadata("iperftarget", serverConfig.ip)
	clientVM.SetStartupScript(string(clientStartup))
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
