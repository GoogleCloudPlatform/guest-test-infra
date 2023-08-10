package gveperf

import (
	"embed"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "network_perf"

// InstanceConfig for setting up test VMs.
type InstanceConfig struct {
	name string
	ip   string
	jfip string
}

var serverConfig = InstanceConfig{name: "server-vm", ip: "192.168.0.4", jfip: "192.168.1.4"}
var clientConfig = InstanceConfig{name: "client-vm", ip: "192.168.0.5", jfip: "192.168.1.5"}

//go:embed startupscripts/*
var scripts embed.FS

const (
	serverStartupScriptURL = "startupscripts/netserver_startup.sh"
	clientStartupScriptURL = "startupscripts/netclient_startup.sh"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if strings.Contains(t.Image, "debian-10") || strings.Contains(t.Image, "rhel-7-7-sap") || strings.Contains(t.Image, "rhel-8-1-sap") {
		// GVNIC is not supported on some older distros.
		fmt.Printf("gVNIC not supported on %v", t.Image)
		return nil
	}

	// Default network.
	defaultNetwork, err := t.CreateNetwork("default-network", false)
	if err != nil {
		return err
	}
	defaultSubnetwork, err := defaultNetwork.CreateSubnetwork("default-subnetwork", "192.168.0.0/24")
	if err != nil {
		return err
	}
	if err := defaultNetwork.CreateFirewallRule("default-allow-tcp", "tcp", []string{"5001", "5201"}, []string{"192.168.0.0/24"}); err != nil {
		return err
	}

	// Jumbo frames network.
	jfNetwork, err := t.CreateNetworkWithMTU("jf-network", imagetest.JumboFramesMTU, false)
	if err != nil {
		return err
	}
	jfSubnetwork, err := jfNetwork.CreateSubnetwork("jf-subnetwork", "192.168.1.0/24")
	if err != nil {
		return err
	}
	if err := jfNetwork.CreateFirewallRule("jf-allow-tcp", "tcp", []string{"5001", "5201"}, []string{"192.168.1.0/24"}); err != nil {
		return err
	}

	// Get the startup scripts as byte arrays.
	serverStartup, err := scripts.ReadFile(serverStartupScriptURL)
	if err != nil {
		return err
	}
	clientStartup, err := scripts.ReadFile(clientStartupScriptURL)
	if err != nil {
		return err
	}

	// Create two VMs for default GVNIC performance testing.
	serverVM, err := t.CreateTestVM(serverConfig.name)
	if err != nil {
		return err
	}
	if err := serverVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
		return err
	}
	if err := serverVM.SetPrivateIP(defaultNetwork, serverConfig.ip); err != nil {
		return err
	}
	if err := serverVM.AddCustomNetwork(jfNetwork, jfSubnetwork); err != nil {
		return err
	}
	if err := serverVM.SetPrivateIP(jfNetwork, serverConfig.jfip); err != nil {
		return err
	}
	serverVM.SetStartupScript(string(serverStartup))

	clientVM, err := t.CreateTestVM(clientConfig.name)
	if err != nil {
		return err
	}
	if err := clientVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
		return err
	}
	if err := clientVM.SetPrivateIP(defaultNetwork, clientConfig.ip); err != nil {
		return err
	}
	if err := clientVM.AddCustomNetwork(jfNetwork, jfSubnetwork); err != nil {
		return err
	}
	if err := clientVM.SetPrivateIP(jfNetwork, clientConfig.jfip); err != nil {
		return err
	}
	clientVM.AddMetadata("enable-guest-attributes", "TRUE")
	clientVM.AddMetadata("default-iperftarget", serverConfig.ip)
	clientVM.AddMetadata("jf-iperftarget", serverConfig.jfip)
	clientVM.SetStartupScript(string(clientStartup))

	// Run tests.
	clientVM.UseGVNIC()
	serverVM.UseGVNIC()
	serverVM.RunTests("TestGVNICExists")
	clientVM.RunTests("TestGVNICExists|TestGVNICPerformance")

	return nil
}
