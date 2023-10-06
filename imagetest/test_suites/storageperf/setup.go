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

const (
	linuxInstallFioScriptURL   = "startupscripts/install_fio.sh"
	windowsInstallFioScriptURL = "startupscripts/install_fio.ps1"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if bootdiskSizeGB == mountdiskSizeGB {
		return fmt.Errorf("boot disk and mount disk must be different sizes for disk identification")
	}
	// initialize vm with the hard coded machine type and platform corresponding to the index
	paramMaps := []map[string]string{
		{"machineType": "c3-standard-88", "diskType": imagetest.HyperdiskExtreme, "metric": "C3_CPUS"},
		// temporarily disable c3d hyperdisk until the api allows it again
		// {"machineType": "c3d-standard-180", "zone": "us-east4-c", "diskType": imagetest.HyperdiskExtreme, "metric": "CPUS"}, // No public metric for this yet but the CPU count will work because they're so large
		{"machineType": "n2-standard-80", "diskType": imagetest.HyperdiskExtreme, "metric": "N2_CPUS"},
		{"machineType": "c3-standard-88", "diskType": imagetest.PdBalanced, "metric": "C3_CPUS"},
		{"machineType": "c3d-standard-180", "zone": "us-east4-c", "diskType": imagetest.PdBalanced, "metric": "CPUS"},
		{"machineType": "n2d-standard-64", "diskType": imagetest.PdBalanced, "metric": "N2D_CPUS"},
		{"machineType": "n1-standard-64", "diskType": imagetest.PdBalanced, "minCpuPlatform": "Intel Skylake", "metric": "CPUS"}, // N1 CPUS aren't measured separately
		// zone for h3 is a temporary measure while h3 is pre-GA
		{"machineType": "h3-standard-88", "zone": "us-central1-a", "diskType": imagetest.PdBalanced, "metric": "CPUS"}, // No public metric
	}
	testVMs := []*imagetest.TestVM{}
	for _, paramMap := range paramMaps {
		machineType := "n1-standard-1"
		if machineTypeParam, foundKey := paramMap["machineType"]; foundKey {
			machineType = machineTypeParam
		}
		// this is the type of the disk where performance is tested
		diskType := imagetest.PdBalanced
		if diskTypeParam, foundKey := paramMap["diskType"]; foundKey {
			diskType = diskTypeParam
		}
		if skipMachineTypeImage(machineType, t.Image) {
			continue
		}

		quota := &daisy.QuotaAvailable{Metric: paramMap["metric"], Region: paramMap["zone"]}
		if len(quota.Region) > 2 {
			quota.Region = quota.Region[:len(quota.Region)-2]
		}
		i, err := strconv.ParseFloat(regexp.MustCompile("-[0-9]+$").FindString(machineType)[1:], 64)
		if err != nil {
			return err
		}
		quota.Units = i
		if err := t.WaitForVMQuota(quota); err != nil {
			return err
		}

		bootDisk := compute.Disk{Name: vmName + machineType + diskType, Type: imagetest.PdBalanced, SizeGb: bootdiskSizeGB}
		mountDisk := compute.Disk{Name: mountDiskName + machineType + diskType, Type: diskType, SizeGb: mountdiskSizeGB}
		bootDisk.Zone = paramMap["zone"]
		mountDisk.Zone = paramMap["zone"]

		vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&bootDisk, &mountDisk}, paramMap)
		if err != nil {
			return err
		}

		vm.AddMetadata("enable-guest-attributes", "TRUE")
		// set the expected performance values
		var vmPerformanceTargets PerformanceTargets
		var foundKey bool = false
		if diskType == imagetest.HyperdiskExtreme {
			vmPerformanceTargets, foundKey = hyperdiskIOPSMap[machineType]
		} else if diskType == imagetest.PdBalanced {
			vmPerformanceTargets, foundKey = pdbalanceIOPSMap[machineType]
		}
		if !foundKey {
			return fmt.Errorf("expected performance for machine type %s and disk type %s not found", machineType, diskType)
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
func skipMachineTypeImage(machineType string, image *compute.Image) bool {
	if strings.HasPrefix(machineType, "c3d") && (strings.Contains(image.Name, "windows-2012") || strings.Contains(image.Name, "windows-2016")) {
		return true
	}

	if strings.Contains(image.Name, "ubuntu-pro-1604") && strings.HasPrefix(machineType, "c3-") {
		return true
	}

	// Fetching the MachineType won't help us, they don't have any architecture
	// property yet.
	if image.Architecture == "ARM64" && !strings.HasPrefix(machineType, "t2a") {
		return true
	}

	// The only way to determine what kind of network a machine type will have is
	// to create an instance of it and inspect the NICs, so we can't get a list of
	// machine types with GVNIC from the API.
	gvnicMachineTypes := []string{"c3-", "c3d-", "h3-"}
	// skip images that don't support GVNIC on gen 3 machine types
	if !utils.HasFeature(image, "GVNIC") {
		for _, excludedType := range gvnicMachineTypes {
			if strings.Contains(machineType, excludedType) {
				return true
			}
		}
	}
	return false
}
