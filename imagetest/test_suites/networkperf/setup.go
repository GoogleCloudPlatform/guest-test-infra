package networkperf

import (
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
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

//go:embed startupscripts/*
var scripts embed.FS

//go:embed targets/*
var targets embed.FS

const (
	serverStartupScriptURL        = "startupscripts/netserver_startup.sh"
	clientStartupScriptURL        = "startupscripts/netclient_startup.sh"
	windowsServerStartupScriptURL = "startupscripts/windows_serverstartup.ps1"
	windowsClientStartupScriptURL = "startupscripts/windows_clientstartup.ps1"
	targetsURL                    = "targets/default_targets.txt"
	tier1TargetsURL               = "targets/tier1_targets.txt"
)

// getExpectedPerf gets the expected performance of the given machine type. Since the targets map only contains breakpoints in vCPUs at which
// each machine type's expected performance changes, find the highest breakpoint at which the expected performance would change, then return
// the performance at said breakpoint.
func getExpectedPerf(targetMap map[string]int, machineType string) (int, error) {
	// Return if already at breakpoint.
	perf, found := targetMap[machineType]
	if found {
		return perf, nil
	}

	machineTypeSplit := strings.Split(machineType, "-")
	family := machineTypeSplit[0]
	familyType := machineTypeSplit[1]
	numCPUs, err := strconv.Atoi(machineTypeSplit[2])
	fmt.Printf("Current cpu: %v", numCPUs)
	if err != nil {
		return 0, nil
	}

	// Decrement numCPUs until a breakpoint is found.
	for !found {
		numCPUs--
		perf, found = targetMap[strings.Join([]string{family, familyType, fmt.Sprint(numCPUs)}, "-")]
		if !found && numCPUs <= 1 {
			return 0, fmt.Errorf("Error: appropriate perf target not found for %v", machineType)
		}
	}
	return perf, nil
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if !utils.HasFeature(t.Image, "GVNIC") {
		t.Skip(fmt.Sprintf("%s does not support GVNIC", t.Image.Name))
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
	if err := defaultNetwork.CreateFirewallRule("default-allow-tcp", "tcp", []string{"5001"}, []string{"192.168.0.0/24"}); err != nil {
		return err
	}

	// Jumbo frames network.
	jfNetwork, err := t.CreateNetwork("jf-network", false)
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
	jfNetwork.SetMTU(imagetest.JumboFramesMTU)

	// Get the targets.
	var defaultPerfTargets map[string]int
	defaultPerfTargetsString, err := targets.ReadFile(targetsURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(defaultPerfTargetsString, &defaultPerfTargets); err != nil {
		return err
	}
	defaultPerfTargetInt, err := getExpectedPerf(defaultPerfTargets, t.MachineType.Name)
	if err != nil {
		return err
	}
	defaultPerfTarget := fmt.Sprint(defaultPerfTargetInt)

	// Default VMs.
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
	clientVM.AddMetadata("expectedperf", defaultPerfTarget)

	// Jumbo frames VMs.
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
	jfClientVM.AddMetadata("expectedperf", defaultPerfTarget)

	// Set startup scripts.
	var serverStartup string
	var clientStartup string
	if utils.HasFeature(t.Image, "WINDOWS") {
		serverStartupByteArr, err := scripts.ReadFile(windowsServerStartupScriptURL)
		if err != nil {
			return err
		}
		clientStartupByteArr, err := scripts.ReadFile(windowsClientStartupScriptURL)
		if err != nil {
			return err
		}
		serverStartup := string(serverStartupByteArr)
		clientStartup := string(clientStartupByteArr)

		serverVM.SetWindowsStartupScript(serverStartup)
		clientVM.SetWindowsStartupScript(clientStartup)
		jfServerVM.SetWindowsStartupScript(serverStartup)
		jfClientVM.SetWindowsStartupScript(clientStartup)
	} else {
		serverStartupByteArr, err := scripts.ReadFile(serverStartupScriptURL)
		if err != nil {
			return err
		}
		clientStartupByteArr, err := scripts.ReadFile(clientStartupScriptURL)
		if err != nil {
			return err
		}
		serverStartup := string(serverStartupByteArr)
		clientStartup := string(clientStartupByteArr)

		serverVM.SetStartupScript(serverStartup)
		clientVM.SetStartupScript(clientStartup)
		jfServerVM.SetStartupScript(serverStartup)
		jfClientVM.SetStartupScript(clientStartup)
	}
	clientVM.UseGVNIC()
	serverVM.UseGVNIC()
	jfClientVM.UseGVNIC()
	jfServerVM.UseGVNIC()

	// Run default tests.
	serverVM.RunTests("TestGVNICExists")
	clientVM.RunTests("TestGVNICExists|TestNetworkPerformance")
	jfServerVM.RunTests("TestGVNICExists")
	jfClientVM.RunTests("TestGVNICExists|TestNetworkPerformance")

	// Check if machine type is valid for tier1 testing.
	mt := t.MachineType.Name
	if !strings.Contains(mt, "n2") && !strings.Contains(mt, "c2") && !strings.Contains(mt, "c3") && !strings.Contains(mt, "m3") {
		// Must be N2, N2D, C2, C2D, C3, C3D, or M3 machine types.
		fmt.Printf("%v: Skipping tier1 tests - %v not supported\n", t.Image.Name, mt)
		return nil
	}
	numCPUs, err := strconv.Atoi(strings.Split(mt, "-")[2])
	if err != nil {
		return err
	}
	if numCPUs < 30 {
		// Must have at least 30 vCPUs.
		fmt.Printf("%v: Skipping tier1 tests - not enough vCPUs (need at least 30, have %v)\n", t.Image.Name, numCPUs)
		return nil
	}

	// Get Tier1 targets.
	var tier1PerfTargets map[string]int
	tier1PerfTargetsString, err := targets.ReadFile(tier1TargetsURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(tier1PerfTargetsString, &tier1PerfTargets); err != nil {
		return err
	}
	tier1PerfTargetInt, err := getExpectedPerf(tier1PerfTargets, t.MachineType.Name)
	if err != nil {
		return err
	}
	tier1PerfTarget := fmt.Sprint(tier1PerfTargetInt)

	// Tier 1 VMs.
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
	tier1ClientVM.AddMetadata("expectedperf", tier1PerfTarget)

	// Set startup scripts.
	if utils.HasFeature(t.Image, "WINDOWS") {
		tier1ServerVM.SetWindowsStartupScript(serverStartup)
		tier1ClientVM.SetWindowsStartupScript(clientStartup)
	} else {
		tier1ServerVM.SetStartupScript(serverStartup)
		tier1ClientVM.SetStartupScript(clientStartup)
	}
	tier1ClientVM.UseGVNIC()
	tier1ServerVM.UseGVNIC()

	tier1ServerVM.RunTests("TestGVNICExists")
	tier1ClientVM.RunTests("TestGVNICExists|TestNetworkPerformance")

	return nil
}
