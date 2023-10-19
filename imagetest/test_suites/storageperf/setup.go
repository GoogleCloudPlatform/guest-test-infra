package storageperf

import (
	"embed"
	"fmt"
	"regexp"
	"strconv"
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

type storagePerfTest struct {
	machineType    string
	arch           string
	diskType       string
	cpuMetric      string
	minCPUPlatform string
	zone           string
	requiredFeatures []string
}

const (
	linuxInstallFioScriptURL   = "startupscripts/install_fio.sh"
	windowsInstallFioScriptURL = "startupscripts/install_fio.ps1"
)

var storagePerfTestConfig = []storagePerfTest{
	{
		arch:        "X86_64",
		machineType: "h3-standard-88",
		zone:        "us-central1-a",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	/* temporarily disable c3d hyperdisk until the api allows it again
	{
		arch: "X86_64",
		machineType: "c3d-standard-180",
		zone: "us-east4-c",
		diskType: imagetest.HyperdiskExtreme,
		cpuMetric: "CPUS", // No public metric for this yet but the CPU count will work because they're so large
		requiredFeatures: []string{"GVNIC"},
	},*/
	{
		arch:        "X86_64",
		machineType: "c3d-standard-180",
		zone:        "us-east4-c",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		arch:        "X86_64",
		machineType: "c3-standard-88",
		diskType:    imagetest.HyperdiskExtreme,
		cpuMetric:   "C3_CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		arch:        "X86_64",
		machineType: "c3-standard-88",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "C3_CPUS",
		requiredFeatures: []string{"GVNIC"},
	},
	{
		arch:        "ARM64",
		machineType: "t2a-standard-48",
		zone:        "us-central1-a",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "T2A_CPUS",
	},
	{
		arch:        "X86_64",
		machineType: "n2-standard-80",
		diskType:    imagetest.HyperdiskExtreme,
		cpuMetric:   "N2_CPUS",
	},
	{
		arch:        "X86_64",
		machineType: "n2d-standard-64",
		diskType:    imagetest.PdBalanced,
		cpuMetric:   "N2D_CPUS",
	},
	{
		arch:           "X86_64",
		machineType:    "n1-standard-64",
		diskType:       imagetest.PdBalanced,
		minCPUPlatform: "Intel Skylake",
		cpuMetric:      "CPUS",
	},
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if bootdiskSizeGB == mountdiskSizeGB {
		return fmt.Errorf("boot disk and mount disk must be different sizes for disk identification")
	}
	testVMs := []*imagetest.TestVM{}
	for _, tc := range storagePerfTestConfig {
		if skipTest(tc, t.Image) {
			continue
		}

		region := tc.zone
		if len(region) > 2 {
			region = region[:len(region)-2]
		}
		if err := t.WaitForDisksQuota(&daisy.QuotaAvailable{Metric: "SSD_TOTAL_GB", Units: bootdiskSizeGB + mountdiskSizeGB, Region: region}); err != nil {
			return err
		}
		if tc.cpuMetric != "" {
			quota := &daisy.QuotaAvailable{Metric: tc.cpuMetric, Region: region}

			i, err := strconv.ParseFloat(regexp.MustCompile("-[0-9]+$").FindString(tc.machineType)[1:], 64)
			if err != nil {
				return err
			}
			quota.Units = i
			if err := t.WaitForVMQuota(quota); err != nil {
				return err
			}
		}

		bootDisk := compute.Disk{Name: vmName + tc.machineType + tc.diskType, Type: imagetest.PdBalanced, SizeGb: bootdiskSizeGB, Zone: tc.zone}
		mountDisk := compute.Disk{Name: mountDiskName + tc.machineType + tc.diskType, Type: tc.diskType, SizeGb: mountdiskSizeGB, Zone: tc.zone}

		vm, err := t.CreateTestVMMultipleDisks(
			[]*compute.Disk{&bootDisk, &mountDisk},
			map[string]string{"machineType": tc.machineType, "minCpuPlatform": tc.minCPUPlatform, "zone": tc.zone},
		)
		if err != nil {
			return err
		}

		vm.AddMetadata("enable-guest-attributes", "TRUE")
		// set the expected performance values
		var vmPerformanceTargets PerformanceTargets
		var foundKey bool = false
		if tc.diskType == imagetest.HyperdiskExtreme {
			vmPerformanceTargets, foundKey = hyperdiskIOPSMap[tc.machineType]
		} else if tc.diskType == imagetest.PdBalanced {
			vmPerformanceTargets, foundKey = pdbalanceIOPSMap[tc.machineType]
		}
		if !foundKey {
			return fmt.Errorf("expected performance for machine type %s and disk type %s not found", tc.machineType, tc.diskType)
		}
		vm.AddMetadata(randReadAttribute, fmt.Sprintf("%f", vmPerformanceTargets.randReadIOPS))
		vm.AddMetadata(randWriteAttribute, fmt.Sprintf("%f", vmPerformanceTargets.randWriteIOPS))
		vm.AddMetadata(seqReadAttribute, fmt.Sprintf("%f", vmPerformanceTargets.seqReadBW))
		vm.AddMetadata(seqWriteAttribute, fmt.Sprintf("%f", vmPerformanceTargets.seqWriteBW))
		if utils.HasFeature(t.Image, "WINDOWS") {
			windowsStartup, err := scripts.ReadFile(windowsInstallFioScriptURL)
			if err != nil {
				return err
			}
			vm.AddMetadata("windows-startup-script-ps1", string(windowsStartup))
		} else {
			linuxStartup, err := scripts.ReadFile(linuxInstallFioScriptURL)
			if err != nil {
				return err
			}
			vm.SetStartupScript(string(linuxStartup))
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
