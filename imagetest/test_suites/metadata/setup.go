package metadata

import (
	"embed"
	"strings"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
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
	shutdownScriptWindowsURL = "scripts/shutdownScriptWindows.ps1"
	startupScriptWindowsURL  = "scripts/startupScriptWindows.ps1"
	daemonScriptWindowsURL   = "scripts/daemonScriptWindows.ps1"
)

//go:embed *
var scripts embed.FS

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {

	vm, err := t.CreateTestVM("mdscommunication")
	if err != nil {
		return err
	}

	vm2Inst := &daisy.Instance{}
	vm2Inst.Metadata = map[string]string{imagetest.ShouldRebootDuringTest: "true"}
	vm2, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "shutdownscripts"}}, vm2Inst)
	if err != nil {
		return err
	}
	vm2.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm2.Reboot(); err != nil {
		return err
	}

	vm3Inst := &daisy.Instance{}
	vm3Inst.Metadata = map[string]string{imagetest.ShouldRebootDuringTest: "true"}
	vm3, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "shutdownscriptsfailed"}}, vm3Inst)
	if err != nil {
		return err
	}
	vm3.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm3.Reboot(); err != nil {
		return err
	}

	vm4Inst := &daisy.Instance{}
	vm4Inst.Metadata = map[string]string{imagetest.ShouldRebootDuringTest: "true"}
	vm4, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "shutdownurlscripts"}}, vm4Inst)
	if err != nil {
		return err
	}
	vm4.AddMetadata("enable-guest-attributes", "TRUE")
	if err := vm4.Reboot(); err != nil {
		return err
	}

	vm6, err := t.CreateTestVM("startupscripts")
	if err != nil {
		return err
	}
	vm6.AddMetadata("enable-guest-attributes", "TRUE")

	vm7, err := t.CreateTestVM("startupscriptsfailed")
	if err != nil {
		return err
	}
	vm7.AddMetadata("enable-guest-attributes", "TRUE")

	vm8, err := t.CreateTestVM("daemonscripts")
	if err != nil {
		return err
	}
	vm8.AddMetadata("enable-guest-attributes", "TRUE")

	var startupByteArr []byte
	var shutdownByteArr []byte
	var daemonByteArr []byte

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
		startupScript := string(startupByteArr)
		shutdownScript := string(shutdownByteArr)
		daemonScript := string(daemonByteArr)

		vm2.SetWindowsShutdownScript(shutdownScript)
		vm3.SetWindowsShutdownScript(strings.Repeat("a", metadataMaxLength))
		vm4.SetWindowsShutdownScriptURL(shutdownScript)
		vm6.SetWindowsStartupScript(startupScript)
		vm7.SetWindowsStartupScript(strings.Repeat("a", metadataMaxLength))
		vm8.SetWindowsStartupScript(daemonScript)

		sysprepspecialize, err := t.CreateTestVM("sysprepspecialize")
		if err != nil {
			return err
		}
		sysprepspecialize.AddMetadata("enable-guest-attributes", "TRUE")
		sysprepspecialize.AddMetadata("sysprep-specialize-script-cmd", `pwsh -Command "Invoke-RestMethod -Method Put -Body startup_success -Headers @{'Metadata-Flavor' = 'Google'} -Uri 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/result' -ContentType 'application/json; charset=utf-8' -UseBasicParsing"`)
		sysprepspecialize.RunTests("TestSysprepSpecialize")

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
		startupScript := string(startupByteArr)
		shutdownScript := string(shutdownByteArr)
		daemonScript := string(daemonByteArr)

		vm2.SetShutdownScript(shutdownScript)
		vm3.SetShutdownScript(strings.Repeat("a", metadataMaxLength))
		vm4.SetShutdownScriptURL(shutdownScript)
		vm6.SetStartupScript(startupScript)
		vm7.SetStartupScript(strings.Repeat("a", metadataMaxLength))
		vm8.SetStartupScript(daemonScript)
	}

	// Run the tests after setup is complete.
	vm.RunTests("TestTokenFetch|TestMetaDataResponseHeaders|TestGetMetaDataUsingIP")
	vm2.RunTests("TestShutdownScripts")
	vm3.RunTests("TestShutdownScriptsFailed")
	vm4.RunTests("TestShutdownURLScripts")
	vm6.RunTests("TestStartupScripts")
	vm7.RunTests("TestStartupScriptsFailed")
	vm8.RunTests("TestDaemonScripts")

	return nil
}
