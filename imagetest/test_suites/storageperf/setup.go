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
	hyperdiskParams := map[string]string{"machineType": "c3-standard-88"}
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: vmName, Type: imagetest.PdBalanced, SizeGb: bootdiskSize},
		{Name: mountDiskName, Type: imagetest.HyperdiskExtreme, SizeGb: hyperdiskSize}}, hyperdiskParams)
	if err != nil {
		return err
	}

	vm.AddMetadata("enable-guest-attributes", "TRUE")
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
	vm.RunTests("TestReadIOPS|TestWriteIOPS")
	return nil
}
