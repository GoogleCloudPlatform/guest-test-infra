package metadata

import (
	"embed"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	// max metadata value 256kb https://cloud.google.com/compute/docs/metadata/setting-custom-metadata#limitations
	// metadataMaxLength = 256 * 1024
	// TODO(hopkiw): above is commented out until error handler is added to
	// output scanner in the script runner. Use smaller size for now.
	metadataMaxLength        = 32768
	shutdownScriptLinuxURL   = "scripts/shutdownScriptLinux.sh"
	startupScriptLinuxURL    = "scripts/startupScriptLinux.sh"
	daemonScriptLinuxURL     = "scripts/daemonScriptLinux.sh"
	timeScriptLinuxURL       = "scripts/shutdownTimeLinux.sh"
	shutdownScriptWindowsURL = "scripts/shutdownScriptWindows.ps1"
	startupScriptWindowsURL  = "scripts/startupScriptWindows.ps1"
	daemonScriptWindowsURL   = "scripts/daemonScriptWindows.ps1"
	timeScriptWindowsURL     = "scripts/shutdownTimeWindows.ps1"
)

//go:embed *
var scripts embed.FS

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {

	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}

	shutdownScriptParams := map[string]string{imagetest.ShouldRebootDuringTest: "true"}
	vm2, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "vm2"}}, shutdownScriptParams)
	if err != nil {
		return err
	}
	vm2.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm2.Reboot(); err != nil {
		return err
	}

	vm3, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "vm3"}}, shutdownScriptParams)
	if err != nil {
		return err
	}
	vm3.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm3.Reboot(); err != nil {
		return err
	}

	vm4, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "vm4"}}, shutdownScriptParams)
	if err != nil {
		return err
	}
	vm4.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm4.Reboot(); err != nil {
		return err
	}

	vm5, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "vm5"}}, shutdownScriptParams)
	if err != nil {
		return err
	}
	vm5.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm5.Reboot(); err != nil {
		return err
	}

	vm6, err := t.CreateTestVM("vm6")
	if err != nil {
		return err
	}
	vm6.AddMetadata("enable-guest-attributes", "TRUE")

	vm7, err := t.CreateTestVM("vm7")
	if err != nil {
		return err
	}
	vm7.AddMetadata("enable-guest-attributes", "TRUE")

	vm8, err := t.CreateTestVM("vm8")
	if err != nil {
		return err
	}
	vm8.AddMetadata("enable-guest-attributes", "TRUE")

	var startupByteArr []byte
	var shutdownByteArr []byte
	var daemonByteArr []byte
	var timeByteArr []byte

	// Determine if the OS is Windows or Linux and set the appropriate script metadata.
	if utils.HasFeature(t.Image, "WINDOWS") {
		startupByteArr, err = scripts.ReadFile(startupScriptWindowsURL)
		if err != nil {
			return err
		}
		shutdownByteArr, err = scripts.ReadFile(shutdownScriptWindowsURL)
		if err != nil {
			return err
		}
		daemonByteArr, err = scripts.ReadFile(daemonScriptWindowsURL)
		if err != nil {
			return err
		}
		timeByteArr, err = scripts.ReadFile(timeScriptWindowsURL)
		if err != nil {
			return err
		}
		startupScript := string(startupByteArr)
		shutdownScript := string(shutdownByteArr)
		daemonScript := string(daemonByteArr)
		timeScript := string(timeByteArr)

		vm2.SetWindowsShutdownScript(shutdownScript)
		vm3.SetWindowsShutdownScript(strings.Repeat("a", metadataMaxLength))
		vm4.SetWindowsShutdownScriptURL(shutdownScript)
		vm5.SetWindowsShutdownScript(timeScript)
		vm6.SetWindowsStartupScript(startupScript)
		vm7.SetWindowsStartupScript(strings.Repeat("a", metadataMaxLength))
		vm8.SetWindowsStartupScript(daemonScript)

	} else {
		startupByteArr, err = scripts.ReadFile(startupScriptLinuxURL)
		if err != nil {
			return err
		}
		shutdownByteArr, err = scripts.ReadFile(shutdownScriptLinuxURL)
		if err != nil {
			return err
		}
		daemonByteArr, err = scripts.ReadFile(daemonScriptLinuxURL)
		if err != nil {
			return err
		}
		timeByteArr, err = scripts.ReadFile(timeScriptWindowsURL)
		if err != nil {
			return err
		}
		startupScript := string(startupByteArr)
		shutdownScript := string(shutdownByteArr)
		daemonScript := string(daemonByteArr)
		timeScript := string(timeByteArr)

		vm2.SetShutdownScript(shutdownScript)
		vm3.SetShutdownScript(strings.Repeat("a", metadataMaxLength))
		vm4.SetShutdownScriptURL(shutdownScript)
		vm5.SetShutdownScript(timeScript)
		vm6.SetStartupScript(startupScript)
		vm7.SetStartupScript(strings.Repeat("a", metadataMaxLength))
		vm8.SetStartupScript(daemonScript)
	}

	// Run the tests after setup is complete.
	vm.RunTests("TestTokenFetch|TestMetaDataResponseHeaders|TestGetMetaDataUsingIP")
	vm2.RunTests("TestShutdownScripts")
	vm3.RunTests("TestShutdownScriptsFailed")
	vm4.RunTests("TestShutdownURLScripts")
	vm5.RunTests("TestShutdownScriptTime")
	vm6.RunTests("TestStartupScripts")
	vm7.RunTests("TestStartupScriptsFailed")
	vm8.RunTests("TestDaemonScripts")

	return nil
}
