package storageperf

import (
	"embed"
	"flag"
	"fmt"
	"regexp"
	"strings"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "storageperf"

//go:embed startupscripts/*
var scripts embed.FS

var testFilter = flag.String("storageperf_test_filter", ".*", "regexp filter for storageperf test cases, only cases with a matching name will be run")

type storagePerfTest struct {
	name             string
	machineType      string
	arch             string
	diskType         string
	cpuMetric        string
	minCPUPlatform   string
	zone             string
	requiredFeatures []string
}

const (
	linuxInstallFioScriptURL   = "startupscripts/install_fio.sh"
	windowsInstallFioScriptURL = "startupscripts/install_fio.ps1"
)

var storagePerfTestConfig = []storagePerfTest{
	{
		name:             "h3-pd",
		arch:             "X86_64",
		machineType:      "h3-standard-88",
		zone:             "us-central1-a",
		diskType:         imagetest.PdBalanced,
		cpuMetric:        "CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	/* temporarily disable c3d hyperdisk until the api allows it again
	{
		name: "c3d-hde",
		arch: "X86_64",
		machineType: "c3d-standard-180",
		zone: "us-east4-c",
		diskType: imagetest.HyperdiskExtreme,
		cpuMetric: "CPUS", // No public metric for this yet but the CPU count will work because they're so large
		requiredFeatures: []string{"GVNIC"},
	},*/
	{
		name:             "c3d-pd",
		arch:             "X86_64",
		machineType:      "c3d-standard-180",
		zone:             "us-east4-c",
		diskType:         imagetest.PdBalanced,
		cpuMetric:        "CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		name:             "c3-lssd",
		arch:             "X86_64",
		machineType:      "c3-standard-88-lssd",
		diskType:         "lssd",
		cpuMetric:        "C3_CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		name:             "c3-hde",
		arch:             "X86_64",
		machineType:      "c3-standard-88",
		diskType:         imagetest.HyperdiskExtreme,
		cpuMetric:        "C3_CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		name:             "c3-pd",
		arch:             "X86_64",
		machineType:      "c3-standard-88",
		diskType:         imagetest.PdBalanced,
		cpuMetric:        "C3_CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		name:             "c4-hdb",
		zone:             "us-east5-b",
		arch:             "X86_64",
		machineType:      "c4-standard-192",
		diskType:         imagetest.HyperdiskBalanced,
		cpuMetric:        "CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		name:             "c4-hde",
		zone:             "us-east5-b",
		arch:             "X86_64",
		machineType:      "c4-standard-192",
		diskType:         imagetest.HyperdiskExtreme,
		cpuMetric:        "CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		name:        "t2a-pd",
		arch:        "ARM64",
		machineType: "t2a-standard-48",
		zone:        "us-central1-a",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "T2A_CPUS",
	},
	{
		name:             "n4-hdb",
		arch:             "X86_64",
		machineType:      "n4-standard-64",
		cpuMetric:        "CPUS",
		requiredFeatures: []string{"GVNIC"},
		diskType:         imagetest.HyperdiskBalanced,
		zone:             "us-east4-b",
	},
	{
		name:        "n2-hde",
		arch:        "X86_64",
		machineType: "n2-standard-80",
		diskType:    imagetest.HyperdiskExtreme,
		cpuMetric:   "N2_CPUS",
	},
	{
		name:        "n2d-pd",
		arch:        "X86_64",
		machineType: "n2d-standard-64",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "N2D_CPUS",
	},
	{
		name:           "n1-pd",
		arch:           "X86_64",
		machineType:    "n1-standard-64",
		diskType:       imagetest.PdBalanced,
		minCPUPlatform: "Intel Skylake",
		cpuMetric:      "CPUS",
	},
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	filter, err := regexp.Compile(*testFilter)
	if err != nil {
		return fmt.Errorf("invalid test case filter: %v", err)
	}
	testVMs := []*imagetest.TestVM{}
	for _, tc := range storagePerfTestConfig {
		if skipTest(tc, t.Image) || !filter.MatchString(tc.name) {
			continue
		}

		region := tc.zone
		if len(region) > 2 {
			region = region[:len(region)-2]
		}

		mountdiskSizeGB := getRequiredDiskSize(tc.machineType, tc.diskType)
		// disk sizes must be different for disk identification
		if bootdiskSizeGB == mountdiskSizeGB {
			mountdiskSizeGB++
		}
		bootDisk := &compute.Disk{Name: vmName + tc.machineType + tc.diskType, Type: imagetest.PdBalanced, SizeGb: bootdiskSizeGB, Zone: tc.zone}
		disks := []*compute.Disk{bootDisk}

		if tc.diskType != "lssd" {
			if err := t.WaitForDisksQuota(&daisy.QuotaAvailable{Metric: "SSD_TOTAL_GB", Units: float64(bootdiskSizeGB + mountdiskSizeGB), Region: region}); err != nil {
				return err
			}
			if tc.cpuMetric != "" {
				quota := &daisy.QuotaAvailable{Metric: tc.cpuMetric, Region: region}
				z := tc.zone
				if z == "" {
					z = t.Zone.Name
				}
				mt, err := t.Client.GetMachineType(t.Project.Name, z, tc.machineType)
				if err != nil {
					return fmt.Errorf("could not find machinetype %v", err)
				}
				quota.Units = float64(mt.GuestCpus)
				if err := t.WaitForVMQuota(quota); err != nil {
					return err
				}
			}

			mountDisk := &compute.Disk{Name: mountDiskName + tc.machineType + tc.diskType, Type: tc.diskType, SizeGb: mountdiskSizeGB, Zone: tc.zone}
			disks = append(disks, mountDisk)
		}

		daisyInst := &daisy.Instance{}
		daisyInst.MachineType = tc.machineType
		daisyInst.MinCpuPlatform = tc.minCPUPlatform
		daisyInst.Zone = tc.zone
		vm, err := t.CreateTestVMMultipleDisks(disks, daisyInst)
		if err != nil {
			return err
		}

		vm.AddMetadata("enable-guest-attributes", "TRUE")
		// set the disk type: hyperdisk has different testing parameters from https://cloud.google.com/compute/docs/disks/benchmark-hyperdisk-performance
		vm.AddMetadata(diskTypeAttribute, tc.diskType)
		vm.AddMetadata(diskSizeGBAttribute, fmt.Sprintf("%d", mountdiskSizeGB))
		// set the expected performance values
		var vmPerformanceTargets PerformanceTargets
		var foundKey bool = false
		if tc.diskType == imagetest.HyperdiskExtreme {
			vmPerformanceTargets, foundKey = hyperdiskExtremeIOPSMap[tc.machineType]
		} else if tc.diskType == imagetest.HyperdiskBalanced {
			vmPerformanceTargets, foundKey = hyperdiskBalancedIOPSMap[tc.machineType]
		} else if tc.diskType == imagetest.HyperdiskThroughput {
			vmPerformanceTargets, foundKey = hyperdiskThroughputIOPSMap[tc.machineType]
		} else if tc.diskType == imagetest.PdBalanced {
			vmPerformanceTargets, foundKey = pdbalanceIOPSMap[tc.machineType]
		} else if tc.diskType == "lssd" {
			vmPerformanceTargets, foundKey = lssdIOPSMap[tc.machineType]
		}
		if !foundKey {
			return fmt.Errorf("expected performance for machine type %s and disk type %s not found", tc.machineType, tc.diskType)
		}
		vm.AddMetadata(randReadAttribute, fmt.Sprintf("%f", vmPerformanceTargets.randReadIOPS))
		vm.AddMetadata(randWriteAttribute, fmt.Sprintf("%f", vmPerformanceTargets.randWriteIOPS))
		vm.AddMetadata(seqReadAttribute, fmt.Sprintf("%f", vmPerformanceTargets.seqReadBW))
		vm.AddMetadata(seqWriteAttribute, fmt.Sprintf("%f", vmPerformanceTargets.seqWriteBW))
		// for now, only use the startup script on windows because the linux startup script to install fio can take a while, leading to race conditions.
		if utils.HasFeature(t.Image, "WINDOWS") {
			windowsStartup, err := scripts.ReadFile(windowsInstallFioScriptURL)
			if err != nil {
				return err
			}
			vm.AddMetadata("windows-startup-script-ps1", string(windowsStartup))
		}
		testVMs = append(testVMs, vm)
	}
	for _, vm := range testVMs {
		vm.RunTests("TestRandomReadIOPS|TestSequentialReadIOPS|TestRandomWriteIOPS|TestSequentialWriteIOPS")
	}
	return nil
}

// Due to compatability issues or other one off errors, some combinations of machine types and images must be skipped. This function returns true for those combinations.
func skipTest(tc storagePerfTest, image *compute.Image) bool {
	if image.Architecture != tc.arch {
		return true
	}
	for _, feat := range tc.requiredFeatures {
		if !utils.HasFeature(image, feat) {
			return true
		}
	}
	if strings.HasPrefix(tc.machineType, "c3d") && (strings.Contains(image.Family, "windows-2012") || strings.Contains(image.Family, "windows-2016")) {
		return true // Skip c3d on older windows
	}
	if strings.Contains(image.Name, "ubuntu-pro-1604") && strings.HasPrefix(tc.machineType, "c3-") {
		return true // Skip c3 on older ubuntu
	}
	return false
}
