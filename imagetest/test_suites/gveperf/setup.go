package gveperf

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "gveperf"

// InstanceConfig for setting up test VMs.
type InstanceConfig struct {
	name string
	ip   string
}

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

	if strings.Contains(t.Image, "debian-10") || strings.Contains(t.Image, "rhel-7-7-sap") || strings.Contains(t.Image, "rhel-8-1-sap") {
		// GVNIC is not supported on some older distros.
		return fmt.Errorf("GVNIC is not supported on %v", t.Image)
	}
	clientVM.UseGVNIC()
	serverVM.UseGVNIC()
	clientVM.RunTests("TestGVNIC")
	return nil
}
