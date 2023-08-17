package gveperf

import (
	"embed"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "network_perf"

// InstanceConfig for setting up test VMs.
type InstanceConfig struct {
	name string
	ip   string
}

var serverConfig = InstanceConfig{name: "server-vm", ip: "192.168.0.4"}
var clientConfig = InstanceConfig{name: "client-vm", ip: "192.168.0.5"}
var jfServerConfig = InstanceConfig{name: "jf-server-vm", ip: "192.168.1.4"}
var jfClientConfig = InstanceConfig{name: "jf-client-vm", ip: "192.168.1.5"}

//go:embed startupscripts/*
var scripts embed.FS

const (
	serverStartupScriptURL = "startupscripts/netserver_startup.sh"
	clientStartupScriptURL = "startupscripts/netclient_startup.sh"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// Default network.
	defaultNetwork, err := t.CreateNetwork("default-network", false)
	if err != nil {
		return err
	}
	defaultSubnetwork, err := defaultNetwork.CreateSubnetwork("default-subnetwork", "192.168.0.0/24")
	if err != nil {
		return err
	}
	if err := defaultNetwork.CreateFirewallRule("default-allow-tcp", "tcp", []string{"5001"}, []string{"192.168.0.0/24"}); err != nil {
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
	if err := jfNetwork.CreateFirewallRule("jf-allow-tcp", "tcp", []string{"5001"}, []string{"192.168.1.0/24"}); err != nil {
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
	clientVM.AddMetadata("enable-guest-attributes", "TRUE")
	clientVM.AddMetadata("iperftarget", serverConfig.ip)
	clientVM.SetStartupScript(string(clientStartup))

	// Jumbo frames VMs
	jfServerVM, err := t.CreateTestVM(jfServerConfig.name)
	if err != nil {
		return err
	}
	if err := jfServerVM.AddCustomNetwork(jfNetwork, jfSubnetwork); err != nil {
		return err
	}
	if err := jfServerVM.SetPrivateIP(jfNetwork, jfServerConfig.ip); err != nil {
		return err
	}
	jfServerVM.SetStartupScript(string(serverStartup))

	jfClientVM, err := t.CreateTestVM(jfClientConfig.name)
	if err != nil {
		return err
	}
	if err := jfClientVM.AddCustomNetwork(jfNetwork, jfSubnetwork); err != nil {
		return err
	}
	if err := jfClientVM.SetPrivateIP(jfNetwork, jfClientConfig.ip); err != nil {
		return err
	}
	jfClientVM.AddMetadata("enable-guest-attributes", "TRUE")
	jfClientVM.AddMetadata("iperftarget", jfServerConfig.ip)
	jfClientVM.SetStartupScript(string(clientStartup))

	// Setting up tests to run.
	serverVMTests := ""
	clientVMTests := ""
	if strings.Contains(t.Image, "debian-10") || strings.Contains(t.Image, "rhel-7-7-sap") || strings.Contains(t.Image, "rhel-8-1-sap") {
		// gVNIC not supported on certain images.
	} else {
		clientVM.UseGVNIC()
		serverVM.UseGVNIC()
		jfClientVM.UseGVNIC()
		jfServerVM.UseGVNIC()

		clientVMTests = "TestGVNICExists|"
		serverVMTests = "TestGVNICExists"
	}

	// Run tests.
	serverVM.RunTests(serverVMTests)
	clientVM.RunTests(clientVMTests + "TestNetworkPerformance")
	jfServerVM.RunTests(serverVMTests)
	jfClientVM.RunTests(clientVMTests + "TestNetworkPerformance")

	return nil
}
