package storageperf

import (
	"embed"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
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
	if bootdiskSize == hyperdiskSize {
		return fmt.Errorf("boot disk and mount disk must be different sizes for disk identification")
	}
	// initialize vm with the hard coded machine type and platform corresponding to the index
	paramMaps := []map[string]string{
		{"machineType": "c3-standard-88"},
		{"machineType": "c3d-standard-180", "zone": "us-east4-c"},
		{"machineType": "n2-standard-80"},
	}
	testVMs := []*imagetest.TestVM{}
	for _, paramMap := range paramMaps {
		machineType := "n1-standard-1"
		if machineTypeParam, foundKey := paramMap["machineType"]; foundKey {
			machineType = machineTypeParam
		}
		bootDisk := compute.Disk{Name: vmName + machineType, Type: imagetest.PdBalanced, SizeGb: bootdiskSize}
		mountDisk := compute.Disk{Name: mountDiskName + machineType, Type: imagetest.HyperdiskExtreme, SizeGb: hyperdiskSize}
		bootDisk.Zone = zoneParam
		mountDisk.Zone = zoneParam

		vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{&bootDisk, &mountDisk}, paramMap)
		if err != nil {
			return err
		}

		vm.AddMetadata("enable-guest-attributes", "TRUE")
		// set the expected performance values
		var vmPerformanceTargets PerformanceTargets
		if vmPerformanceTargets, foundKey := expectedIOPSMap[machineType]; !foundKey {
			return fmt.Errorf("expected performance for machine type %s not found", machineType
		}
		vm.AddMetadata(randReadAttribute, vmPerformanceTargets.randReadIOPS)
		vm.AddMetadata(randWriteAttribute, vmPerformanceTargets.randWriteIOPS)
		vm.AddMetadata(seqReadAttribute, vmPerformanceTargets.seqReadIOPS)
		vm.AddMetadata(seqWriteAttribute, vmPerformanceTargets.seqWriteIOPS)
		if strings.Contains(t.Image, "windows") {
			vm.AddMetadata("windowsDriveLetter", windowsDriveLetter)
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
