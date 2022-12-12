package metadata

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	daemonScriptTemplate = `#!/bin/bash
	nohup sleep 3600 > /dev/null 2>&1 < /dev/null &
	echo $! > %s
	`
	daemonOutputPath = "/daemon_out.txt"

	bashScriptTemplate = `#!/bin/bash
	echo "%s" > %s`
	windowsScriptTemplate = `echo "%s" > %s
	ping -n 2 127.0.0.1 >NUL`
	powershellScriptTemplate = `Set-Content -Value "%s" -Path %s
	Start-Sleep 1`
	startupContent  = "The startup script worked."
	shutdownContent = "The shutdown script worked."

	shutdownTimeOutputPath        = "/shutdown.txt"
	windowsShutdownTimeOutputPath = "C:\\shutdown.txt"

	bashShutdownTimeScriptTemplate = `#!/bin/bash
	while [[ 1 ]]; do
	date +%%s >> %s
	  sync
	  sleep 1
	  done`

	windowsShutdownTimeScriptTemplate = `while ($true) {
		$time = (Get-Date).ToString("yyyyMMdd-HHMMss")
		Add-Content -Path %s -Value $time
		Start-Sleep  1
		}`

	// max metadata value 256kb https://cloud.google.com/compute/docs/metadata/setting-custom-metadata#limitations
	// metadataMaxLength = 256 * 1024
	// TODO(hopkiw): above is commented out until error handler is added to
	// output scanner in the script runner. Use smaller size for now.
	metadataMaxLength = 32768
)

type metadataScript struct {
	description    string
	url            bool
	windows        bool
	metadataKey    string
	scriptTemplate string
	outputPath     string
	outputContent  string
}

var windowsMetadataScripts = map[string]metadataScript{
	"sysprep-specialize-script-ps1": {
		description:    "Windows Sysprep Powershell",
		scriptTemplate: powershellScriptTemplate,
		outputPath:     "C:\\sysprep_ps1.txt",
		outputContent:  startupContent,
	},
	"sysprep-specialize-script-cmd": {
		description:    "Windows Sysprep CMD",
		scriptTemplate: windowsScriptTemplate,
		outputPath:     "C:\\sysprep_cmd.txt",
		outputContent:  startupContent,
	},
	"sysprep-specialize-script-bat": {
		description:    "Windows Sysprep BAT",
		scriptTemplate: windowsScriptTemplate,
		outputPath:     "C:\\sysprep_bat.txt",
		outputContent:  startupContent,
	},
	"sysprep-specialize-script-url": {
		description:    "Windows Sysprep URL",
		url:            true,
		scriptTemplate: powershellScriptTemplate,
		outputPath:     "C:\\sysprep_url.txt",
		outputContent:  startupContent,
	},
	"windows-startup-script-ps1": {
		description:    "Windows Startup Powershell",
		scriptTemplate: powershellScriptTemplate,
		outputPath:     "C:\\startup_ps1.txt",
		outputContent:  startupContent,
	},
	"windows-startup-script-cmd": {
		description:    "Windows Startup CMD",
		scriptTemplate: windowsScriptTemplate,
		outputPath:     "C:\\startup_cmd.txt",
		outputContent:  startupContent,
	},
	"windows-startup-script-bat": {
		description:    "Windows Startup BAT",
		scriptTemplate: windowsScriptTemplate,
		outputPath:     "C:\\startup_bat.txt",
		outputContent:  startupContent,
	},
	"windows-shutdown-script-ps1": {
		description:    "Windows shutdown Powershell",
		scriptTemplate: powershellScriptTemplate,
		outputPath:     "C:\\shutdown_ps1.txt",
		outputContent:  shutdownContent,
	},
	"windows-shutdown-script-cmd": {
		description:    "Windows shutdown CMD",
		scriptTemplate: windowsScriptTemplate,
		outputPath:     "C:\\shutdown_cmd.txt",
		outputContent:  shutdownContent,
	},
	"windows-shutdown-script-bat": {
		description:    "Windows shutdown BAT",
		scriptTemplate: windowsScriptTemplate,
		outputPath:     "C:\\shutdown_bat.txt",
		outputContent:  shutdownContent,
	},
	"windows-shutdown-script-url": {
		description:    "Windows shutdown URL",
		url:            true,
		scriptTemplate: powershellScriptTemplate,
		outputPath:     "C:\\shutdown_url.txt",
		outputContent:  shutdownContent,
	},
}

var linuxMetadataScripts = map[string]metadataScript{
	"startup-script": {
		description:    "Linux Startup",
		scriptTemplate: bashScriptTemplate,
		outputPath:     "/startup_out.txt",
		outputContent:  startupContent,
	},
	"shutdown-script": {
		description:    "Linux Shutdown",
		scriptTemplate: bashScriptTemplate,
		outputPath:     "/shutdown_out.txt",
		outputContent:  shutdownContent,
	},
	"shutdown-script-url": {
		description:    "Linux Shutdown URL",
		scriptTemplate: bashScriptTemplate,
		outputPath:     "/shutdown_url.txt",
		outputContent:  shutdownContent,
	},
}

var windowsStartupScriptOrder = []string{
	"sysprep-specialize-script-ps1",
	"sysprep-specialize-script-cmd",
	"sysprep-specialize-script-bat",
	"sysprep-specialize-script-url",
	"windows-startup-script-ps1",
	"windows-startup-script-cmd",
	"windows-startup-script-bat",
}

var windowsShutdownScriptOrder = []string{
	"windows-shutdown-script-ps1",
	"windows-shutdown-script-cmd",
	"windows-shutdown-script-bat",
	"windows-shutdown-script-url",
}

func (ms metadataScript) script() string {
	return fmt.Sprintf(ms.scriptTemplate, ms.outputContent, ms.outputPath)
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	shutdownTimeScript := fmt.Sprintf(bashShutdownTimeScriptTemplate, shutdownTimeOutputPath)
	shutdownTimeMetadataKey := "shutdown-script"
	metadataScripts := linuxMetadataScripts
	if strings.Contains(t.Image, "windows") {
		shutdownTimeScript = fmt.Sprintf(windowsShutdownTimeScriptTemplate, windowsShutdownTimeOutputPath)
		shutdownTimeMetadataKey = "windows-shutdown-script-ps1"
		metadataScripts = windowsMetadataScripts
	}

	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestTokenFetch|TestMetaDataResponseHeaders|TestGetMetaDataUsingIP")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	for key, ms := range metadataScripts {
		if strings.HasSuffix(key, "-url") {
			vm2.SetMetadataScriptURL(key, ms.script())
		} else {
			vm2.AddMetadata(key, ms.script())
		}
	}
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestStartupShutdownScripts|TestWindowsScriptOrder")

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	for key := range metadataScripts {
		vm3.AddMetadata(key, strings.Repeat("a", metadataMaxLength))
	}
	if err := vm3.Reboot(); err != nil {
		return err
	}
	vm3.RunTests("TestStartupShutdownScriptFailed")

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	vm4.AddMetadata(shutdownTimeMetadataKey, shutdownTimeScript)
	if err := vm4.Reboot(); err != nil {
		return err
	}
	vm4.RunTests("TestShutdownScriptTime")

	// Tests here are skipped on Windows, tests after this section are Windows-only.
	if !strings.Contains(t.Image, "windows") {
		vm5, err := t.CreateTestVM("vm5")
		if err != nil {
			return err
		}
		daemonScript := fmt.Sprintf(daemonScriptTemplate, daemonOutputPath)
		vm5.AddMetadata("startup-script", daemonScript)
		vm5.RunTests("TestDaemonScript")

		// Tests after this point are Windows-only
		return nil
	}

	// Windows-only VMs created here

	return nil
}
