package metadata

import (
	"embed"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	// max metadata value 256kb https://cloud.google.com/compute/docs/metadata/setting-custom-metadata#limitations
	// metadataMaxLength = 256 * 1024
	// TODO(hopkiw): above is commented out until error handler is added to
	// output scanner in the script runner. Use smaller size for now.
	metadataMaxLength        = 32768
	shutdownScriptLinuxUrl   = "scripts/shutdownScriptLinux.sh"
	startupScriptLinuxUrl    = "scripts/startupScriptLinux.sh"
	daemonScriptLinuxUrl     = "scripts/daemonScriptLinux.sh"
	timeScriptLinuxUrl       = "scripts/shutdownTimeLinux.sh"
	shutdownScriptWindowsUrl = "scripts/shutdownScriptWindows.ps1"
	startupScriptWindowsUrl  = "scripts/startupScriptWindows.ps1"
	daemonScriptWindowsUrl   = "scripts/daemonScriptWindows.ps1"
	timeScriptWindowsUrl     = "scripts/shutdownTimeWindows.ps1"
)

//go:embed *
var scripts embed.FS

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {

	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	if err := vm2.Reboot(); err != nil {
		return err
	}

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.SetShutdownScript(strings.Repeat("a", metadataMaxLength))
	if err := vm3.Reboot(); err != nil {
		return err
	}

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	if err := vm4.Reboot(); err != nil {
		return err
	}

	vm5, err := t.CreateTestVM("vm5")
	if err != nil {
		return err
	}
	if err := vm5.Reboot(); err != nil {
		return err
	}

	vm6, err := t.CreateTestVM("vm6")
	if err != nil {
		return err
	}

	vm7, err := t.CreateTestVM("vm7")
	if err != nil {
		return err
	}
	vm7.SetStartupScript(strings.Repeat("a", metadataMaxLength))

	vm8, err := t.CreateTestVM("vm8")
	if err != nil {
		return err
	}

	var startupByteArr []byte
	var shutdownByteArr []byte
	var daemonByteArr []byte
	var timeByteArr []byte

	if strings.Contains(t.Image, "windows") {
		startupByteArr, err = scripts.ReadFile(startupScriptWindowsUrl)
		if err != nil {
			return err
		}
		shutdownByteArr, err = scripts.ReadFile(shutdownScriptWindowsUrl)
		if err != nil {
			return err
		}
		daemonByteArr, err = scripts.ReadFile(shutdownScriptWindowsUrl)
		if err != nil {
			return err
		}
		timeByteArr, err = scripts.ReadFile(timeScriptWindowsUrl)
		if err != nil {
			return err
		}
		startupScript := string(startupByteArr)
		shutdownScript := string(shutdownByteArr)
		daemonScript := string(daemonByteArr)
		timeScript := string(timeByteArr)

		vm2.SetShutdownScript(shutdownScript)
		vm4.SetShutdownScriptURL(shutdownScript)
		vm5.SetShutdownScript(timeScript)
		vm6.SetWindowsStartupScript(startupScript)
		vm8.SetWindowsStartupScript(daemonScript)

	} else {
		startupByteArr, err = scripts.ReadFile(startupScriptLinuxUrl)
		if err != nil {
			return err
		}
		shutdownByteArr, err = scripts.ReadFile(shutdownScriptLinuxUrl)
		if err != nil {
			return err
		}
		daemonByteArr, err = scripts.ReadFile(shutdownScriptWindowsUrl)
		if err != nil {
			return err
		}
		timeByteArr, err = scripts.ReadFile(timeScriptWindowsUrl)
		if err != nil {
			return err
		}
		startupScript := string(startupByteArr)
		shutdownScript := string(shutdownByteArr)
		daemonScript := string(daemonByteArr)
		timeScript := string(timeByteArr)

		vm2.SetShutdownScript(shutdownScript)
		vm4.SetShutdownScriptURL(shutdownScript)
		vm5.SetShutdownScript(timeScript)
		vm6.SetStartupScript(startupScript)
		vm8.SetStartupScript(daemonScript)
	}

	vm.RunTests("TestTokenFetch|TestMetaDataResponseHeaders|TestGetMetaDataUsingIP")
	vm2.RunTests("TestShutdownScripts")
	vm3.RunTests("TestShutdownScriptsFailed")
	vm4.RunTests("TestShutdownUrlScripts")
	vm5.RunTests("TestShutdownScriptTime")
	vm6.RunTests("TestStartupScripts")
	vm7.RunTests("TestStartupScriptsFailed")
	vm8.RunTests("TestDaemonScript")
	return nil
}
