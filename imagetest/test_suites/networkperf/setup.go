package networkperf

import (
	"embed"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "networkperf"

// InstanceConfig for setting up test VMs.
type InstanceConfig struct {
	name string
	ip   string
}

var serverConfig = InstanceConfig{name: "server-vm", ip: "192.168.0.4"}
var clientConfig = InstanceConfig{name: "client-vm", ip: "192.168.0.5"}
var jfServerConfig = InstanceConfig{name: "jf-server-vm", ip: "192.168.1.4"}
var jfClientConfig = InstanceConfig{name: "jf-client-vm", ip: "192.168.1.5"}
var tier1ServerConfig = InstanceConfig{name: "tier1-server-vm", ip: "192.168.0.6"}
var tier1ClientConfig = InstanceConfig{name: "tier1-client-vm", ip: "192.168.0.7"}

//go:embed *
var scripts embed.FS

const (
	serverStartupScriptURL        = "startupscripts/netserver_startup.sh"
	clientStartupScriptURL        = "startupscripts/netclient_startup.sh"
	windowsServerStartupScriptURL = "startupscripts/windows_serverstartup.ps1"
	windowsClientStartupScriptURL = "startupscripts/windows_clientstartup.ps1"
	targetsURL                    = "targets.txt"
	tier1TargetsURL               = "tier1_targets.txt"
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

	// Get the targets.
	defaultPerfTargets, err := scripts.ReadFile(targetsURL)
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
	clientVM.AddMetadata("perfmap", string(defaultPerfTargets))

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
	jfClientVM.AddMetadata("perfmap", string(defaultPerfTargets))

	// Setting up tests to run.
	if strings.Contains(t.Image, "debian-10") || strings.Contains(t.Image, "rhel-7-7-sap") || strings.Contains(t.Image, "rhel-8-1-sap") {
		// gVNIC not supported on certain images.
		serverVM.RunTests("TestEmpty")
		clientVM.RunTests("TestEmpty")
		jfServerVM.RunTests("TestEmpty")
		jfClientVM.RunTests("TestEmpty")
		return nil
	}
	// Only images that support gVNIC can run tier1 tests.
	tier1PerfTargets, err := scripts.ReadFile(tier1TargetsURL)
	if err != nil {
		return err
	}

	// Create Test VMs for Tier1 tests.
	tier1ServerVM, err := t.CreateTestVM(tier1ServerConfig.name)
	if err != nil {
		return err
	}
	if err := tier1ServerVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
		return err
	}
	if err := tier1ServerVM.SetPrivateIP(defaultNetwork, tier1ServerConfig.ip); err != nil {
		return err
	}
	tier1ServerVM.SetNetworkPerformanceTier("TIER_1")

	tier1ClientVM, err := t.CreateTestVM(tier1ClientConfig.name)
	if err != nil {
		return err
	}
	if err := tier1ClientVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
		return err
	}
	if err := tier1ClientVM.SetPrivateIP(defaultNetwork, tier1ClientConfig.ip); err != nil {
		return err
	}
	tier1ClientVM.AddMetadata("enable-guest-attributes", "TRUE")
	tier1ClientVM.AddMetadata("iperftarget", tier1ServerConfig.ip)
	tier1ClientVM.AddMetadata("perfmap", string(tier1PerfTargets))

	// Set startup scripts.
	var serverStartupByteArr []byte
	var clientStartupByteArr []byte
	if strings.Contains(t.Image, "windows") {
		serverStartupByteArr, err = scripts.ReadFile(windowsServerStartupScriptURL)
		if err != nil {
			return err
		}
		clientStartupByteArr, err = scripts.ReadFile(windowsClientStartupScriptURL)
		if err != nil {
			return err
		}
		serverStartup := string(serverStartupByteArr)
		clientStartup := string(clientStartupByteArr)

		serverVM.SetWindowsStartupScript(serverStartup)
		clientVM.SetWindowsStartupScript(clientStartup)
		jfServerVM.SetWindowsStartupScript(serverStartup)
		jfClientVM.SetWindowsStartupScript(clientStartup)
		tier1ServerVM.SetWindowsStartupScript(serverStartup)
		tier1ClientVM.SetWindowsStartupScript(clientStartup)
	} else {
		serverStartupByteArr, err = scripts.ReadFile(serverStartupScriptURL)
		if err != nil {
			return err
		}
		clientStartupByteArr, err = scripts.ReadFile(clientStartupScriptURL)
		if err != nil {
			return err
		}
		serverStartup := string(serverStartupByteArr)
		clientStartup := string(clientStartupByteArr)

		serverVM.SetStartupScript(serverStartup)
		clientVM.SetStartupScript(clientStartup)
		jfServerVM.SetStartupScript(serverStartup)
		jfClientVM.SetStartupScript(clientStartup)
		tier1ServerVM.SetStartupScript(serverStartup)
		tier1ClientVM.SetStartupScript(clientStartup)
	}

	clientVM.UseGVNIC()
	serverVM.UseGVNIC()
	jfClientVM.UseGVNIC()
	jfServerVM.UseGVNIC()
	tier1ClientVM.UseGVNIC()
	tier1ServerVM.UseGVNIC()

	// Run tests.
	serverVM.RunTests("TestGVNICExists")
	clientVM.RunTests("TestGVNICExists|TestNetworkPerformance")
	jfServerVM.RunTests("TestGVNICExists")
	jfClientVM.RunTests("TestGVNICExists|TestNetworkPerformance")
	tier1ServerVM.RunTests("TestGVNICExists")
	tier1ClientVM.RunTests("TestGVNICExists|TestNetworkPerformance")

	return nil
}
