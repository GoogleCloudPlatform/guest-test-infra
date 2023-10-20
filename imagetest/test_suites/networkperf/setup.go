package networkperf

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "networkperf"

type networkPerfTest struct {
	machineType string   // Machinetype used for test
	zone        string   // (optional) zone required for machinetype
	arch        string   // arch required for machinetype
	networks    []string // Networks to test (TIER_1 and/or DEFAULT)
	quota       *daisy.QuotaAvailable
}

var networkPerfTestConfig = []networkPerfTest{
	{
		machineType: "n1-standard-2",
		arch:        "X86_84",
		networks:    []string{"DEFAULT"},
		quota:       &daisy.QuotaAvailable{Metric: "CPUS", Units: 8},
	},
	{
		machineType: "n2-standard-2",
		arch:        "X86_84",
		networks:    []string{"DEFAULT"},
		quota:       &daisy.QuotaAvailable{Metric: "N2_CPUS", Units: 8},
	},
	{
		machineType: "n2d-standard-2",
		arch:        "X86_84",
		networks:    []string{"DEFAULT"},
		quota:       &daisy.QuotaAvailable{Metric: "N2D_CPUS", Units: 8},
	},
	{
		machineType: "e2-standard-2",
		arch:        "X86_84",
		networks:    []string{"DEFAULT"},
		quota:       &daisy.QuotaAvailable{Metric: "E2_CPUS", Units: 8},
	},
	{
		machineType: "t2d-standard-1",
		arch:        "X86_84",
		networks:    []string{"DEFAULT"},
		quota:       &daisy.QuotaAvailable{Metric: "T2D_CPUS", Units: 4},
	},
	{
		machineType: "t2a-standard-1",
		arch:        "ARM64",
		networks:    []string{"DEFAULT"},
		zone:        "us-central1-a",
		quota:       &daisy.QuotaAvailable{Metric: "T2A_CPUS", Units: 4, Region: "us-central1"},
	},
	{
		machineType: "n2-standard-32",
		arch:        "X86_64",
		networks:    []string{"DEFAULT", "TIER_1"},
		quota:       &daisy.QuotaAvailable{Metric: "N2_CPUS", Units: 192}, // 32 cpus x 2 vms per tier 1 test + 32 x 4 vms per default test
	},
	{
		machineType: "n2d-standard-48",
		arch:        "X86_64",
		networks:    []string{"DEFAULT", "TIER_1"},
		quota:       &daisy.QuotaAvailable{Metric: "N2D_CPUS", Units: 288},
	},
}

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
func getExpectedPerf(targetMap map[string]int, machineType *compute.MachineType) (int, error) {
	// Return if already at breakpoint.
	perf, found := targetMap[machineType.Name]
	if found {
		return perf, nil
	}

	numCPUs := machineType.GuestCpus

	// Decrement numCPUs until a breakpoint is found.
	for !found {
		numCPUs--
		perf, found = targetMap[regexp.MustCompile("-[0-9]+$").ReplaceAllString(machineType.Name, fmt.Sprintf("-%d", numCPUs))]
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
	for _, tc := range networkPerfTestConfig {
		if tc.arch != t.Image.Architecture {
			continue
		}

		if tc.quota != nil {
			t.WaitForVMQuota(tc.quota)
		}
		machine, err := t.Client.GetMachineType(t.Project.Name, t.Zone.Name, tc.machineType)
		if err != nil {
			return err
		}

		// Create network containing everything
		defaultNetwork, err := t.CreateNetwork("default-network-"+tc.machineType, false)
		if err != nil {
			return err
		}
		defaultSubnetwork, err := defaultNetwork.CreateSubnetwork("default-subnetwork-"+tc.machineType, "192.168.0.0/24")
		if err != nil {
			return err
		}
		if err := defaultNetwork.CreateFirewallRule("default-allow-tcp-"+tc.machineType, "tcp", []string{"5001"}, []string{"192.168.0.0/24"}); err != nil {
			return err
		}
		// Jumbo frames network.
		jfNetwork, err := t.CreateNetwork("jf-network-"+tc.machineType, false)
		if err != nil {
			return err
		}
		jfSubnetwork, err := jfNetwork.CreateSubnetwork("jf-subnetwork-"+tc.machineType, "192.168.1.0/24")
		if err != nil {
			return err
		}
		if err := jfNetwork.CreateFirewallRule("jf-allow-tcp-"+tc.machineType, "tcp", []string{"5001"}, []string{"192.168.1.0/24"}); err != nil {
			return err
		}
		jfNetwork.SetMTU(imagetest.JumboFramesMTU)

		// Read startup scripts
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
			serverStartup = string(serverStartupByteArr)
			clientStartup = string(clientStartupByteArr)
		} else {
			serverStartupByteArr, err := scripts.ReadFile(serverStartupScriptURL)
			if err != nil {
				return err
			}
			clientStartupByteArr, err := scripts.ReadFile(clientStartupScriptURL)
			if err != nil {
				return err
			}
			serverStartup = string(serverStartupByteArr)
			clientStartup = string(clientStartupByteArr)
		}
		for _, net := range tc.networks {
			switch net {
			case "DEFAULT":
				// Get the targets.
				var defaultPerfTargets map[string]int
				defaultPerfTargetsString, err := targets.ReadFile(targetsURL)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(defaultPerfTargetsString, &defaultPerfTargets); err != nil {
					return err
				}
				defaultPerfTargetInt, err := getExpectedPerf(defaultPerfTargets, machine)
				if err != nil {
					return fmt.Errorf("could not get default perf target: %v", err)
				}
				defaultPerfTarget := fmt.Sprint(defaultPerfTargetInt)

				// Default VMs.
				serverDisk := compute.Disk{Name: serverConfig.name + "-" + tc.machineType, Type: imagetest.PdBalanced, Zone: tc.zone}
				serverVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&serverDisk}, nil)
				if err != nil {
					return err
				}
				serverVM.ForceMachineType(tc.machineType)
				serverVM.ForceZone(tc.zone)
				if err := serverVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
					return err
				}
				if err := serverVM.SetPrivateIP(defaultNetwork, serverConfig.ip); err != nil {
					return err
				}

				clientDisk := compute.Disk{Name: clientConfig.name + "-" + tc.machineType, Type: imagetest.PdBalanced, Zone: tc.zone}
				clientVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&clientDisk}, nil)
				if err != nil {
					return err
				}
				clientVM.ForceMachineType(tc.machineType)
				clientVM.ForceZone(tc.zone)
				if err := clientVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
					return err
				}
				if err := clientVM.SetPrivateIP(defaultNetwork, clientConfig.ip); err != nil {
					return err
				}
				clientVM.AddMetadata("enable-guest-attributes", "TRUE")
				clientVM.AddMetadata("iperftarget", serverConfig.ip)
				clientVM.AddMetadata("expectedperf", defaultPerfTarget)
				clientVM.AddMetadata("network-tier", net)

				// Jumbo frames VMs.
				jfServerDisk := compute.Disk{Name: jfServerConfig.name + "-" + tc.machineType, Type: imagetest.PdBalanced, Zone: tc.zone}
				jfServerVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&jfServerDisk}, nil)
				if err != nil {
					return err
				}
				jfServerVM.ForceMachineType(tc.machineType)
				jfServerVM.ForceZone(tc.zone)
				if err := jfServerVM.AddCustomNetwork(jfNetwork, jfSubnetwork); err != nil {
					return err
				}
				if err := jfServerVM.SetPrivateIP(jfNetwork, jfServerConfig.ip); err != nil {
					return err
				}

				jfClientDisk := compute.Disk{Name: jfClientConfig.name + "-" + tc.machineType, Type: imagetest.PdBalanced, Zone: tc.zone}
				jfClientVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&jfClientDisk}, nil)
				jfClientVM.ForceMachineType(tc.machineType)
				jfClientVM.ForceZone(tc.zone)
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
				jfClientVM.AddMetadata("network-tier", net)

				// Set startup scripts.
				if utils.HasFeature(t.Image, "WINDOWS") {
					serverVM.SetWindowsStartupScript(serverStartup)
					clientVM.SetWindowsStartupScript(clientStartup)
					jfServerVM.SetWindowsStartupScript(serverStartup)
					jfClientVM.SetWindowsStartupScript(clientStartup)
				} else {
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
			case "TIER_1":
				if machine.GuestCpus < 30 {
					// Must have at least 30 vCPUs.
					fmt.Printf("%v: Skipping tier1 tests - not enough vCPUs (need at least 30, have %v)\n", t.Image.Name, machine.GuestCpus)
					continue
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
				tier1PerfTargetInt, err := getExpectedPerf(tier1PerfTargets, machine)
				if err != nil {
					return fmt.Errorf("could not get tier 1 perf target: %v", err)
				}
				tier1PerfTarget := fmt.Sprint(tier1PerfTargetInt)

				// Tier 1 VMs.
				t1ServerDisk := compute.Disk{Name: tier1ServerConfig.name + "-" + tc.machineType, Type: imagetest.PdBalanced, Zone: tc.zone}
				tier1ServerVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&t1ServerDisk}, nil)
				if err != nil {
					return err
				}
				tier1ServerVM.ForceMachineType(tc.machineType)
				tier1ServerVM.ForceZone(tc.zone)
				if err := tier1ServerVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
					return err
				}
				if err := tier1ServerVM.SetPrivateIP(defaultNetwork, tier1ServerConfig.ip); err != nil {
					return err
				}
				tier1ServerVM.SetNetworkPerformanceTier("TIER_1")

				t1ClientDisk := compute.Disk{Name: tier1ClientConfig.name + "-" + tc.machineType, Type: imagetest.PdBalanced, Zone: tc.zone}
				tier1ClientVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&t1ClientDisk}, nil)
				if err != nil {
					return err
				}
				tier1ClientVM.ForceMachineType(tc.machineType)
				tier1ClientVM.ForceZone(tc.zone)
				if err := tier1ClientVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
					return err
				}
				if err := tier1ClientVM.SetPrivateIP(defaultNetwork, tier1ClientConfig.ip); err != nil {
					return err
				}
				tier1ClientVM.AddMetadata("enable-guest-attributes", "TRUE")
				tier1ClientVM.AddMetadata("iperftarget", tier1ServerConfig.ip)
				tier1ClientVM.AddMetadata("expectedperf", tier1PerfTarget)
				tier1ClientVM.AddMetadata("network-tier", net)

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
			}
		}
	}
	return nil
}
