//go:build cit
// +build cit

package metadata

import (
	"io/ioutil"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// The designed shutdown limit is 90s. Let's verify it's executed no less than 80s.
const shutdownTime = 80

// TestStartupShutdownScripts tests that all startup/shutdown scripts
// appropriate for the platform ran successfully.
func TestStartupShutdownScripts(t *testing.T) {
	isWindows := runtime.GOOS == "windows"
	for _, ms := range metadataScripts {
		if ms.windows != isWindows {
			continue
		}
		bytes, err := ioutil.ReadFile(ms.outputPath)
		if err != nil {
			t.Fatalf("failed to read %s output %v", ms.outputPath, err)
		}
		output := strings.TrimSpace(string(bytes))
		output = strings.Trim(output, `"`)
		if output != ms.outputContent {
			t.Fatalf(`%s output expected "%s", but actual "%s"`, ms.description, ms.outputContent, output)
		}
	}
}

// TestStartupShutdownScriptFailed tests that a script that fails to execute
// doesn't crash the vm.
func TestStartupShutdownScriptFailed(t *testing.T) {
	for _, script := range metadataScripts {
		if _, err := utils.GetMetadataAttribute(script.metadataKey); err != nil {
			t.Fatalf("Couldn't get %s from metadata: %v", script.metadataKey, err)
		}
	}
}

// TestWindowsScriptOrder tests that Windows startup & shutdown scripts are
// executed in the documented order.
func TestWindowsScriptOrder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Script run order only checked on Windows.")
	}

	var expectedStartupOrder, actualStartupOrder, expectedShutdownOrder, actualShutdownOrder []string

	for _, ms := range metadataScripts {
		if ms.windows {
			outputFile := strings.TrimPrefix(ms.outputPath, "C:\\")
			if strings.HasPrefix(outputFile, "shutdown") {
				expectedShutdownOrder = append(expectedShutdownOrder, outputFile)
			} else {
				expectedStartupOrder = append(expectedStartupOrder, outputFile)
			}
		}
	}

	command := "Get-ChildItem \"C:\\*.txt\" | Sort-Object LastWriteTime | Select-Object Name"

	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Cannot get output file list: %v - %s", err, output.Stderr)
	}
	outputSlice := strings.Split(output.Stdout, "\n")

	for _, outputFile := range outputSlice {
		outputFile = strings.TrimSpace(outputFile)
		switch {
		case strings.HasPrefix(outputFile, "shutdown"):
			actualShutdownOrder = append(actualShutdownOrder, outputFile)
		case strings.HasPrefix(outputFile, "startup"):
			actualStartupOrder = append(actualStartupOrder, outputFile)
		case strings.HasPrefix(outputFile, "sysprep"):
			actualStartupOrder = append(actualStartupOrder, outputFile)
		}
	}

	if !utils.CmpStringSlice(actualShutdownOrder, expectedShutdownOrder) {
		t.Fatalf("Shutdown files not created in expected order:\nExpected:\n%v\nActual:\n%v", expectedShutdownOrder, actualShutdownOrder)
	}

	if !utils.CmpStringSlice(actualStartupOrder, expectedStartupOrder) {
		t.Fatalf("Startup files not created in expected order:\nExpected:\n%v\nActual:\n%v", expectedStartupOrder, actualStartupOrder)
	}
}

// TestDaemonScript tests that a daemon process started by startup script is
// still running in the VM after completion of the startup script.
func TestDaemonScript(t *testing.T) {
	bytes, err := ioutil.ReadFile(daemonOutputPath)
	if err != nil {
		t.Fatalf("failed to read daemon script PID file: %v", err)
	}
	pid := strings.TrimSpace(string(bytes))
	cmd := exec.Command("ps", "-p", pid)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("command \"ps -p %s\" failed: %v, output was: %s", pid, err, out)
		t.Fatalf("Daemon process not running")
	}
}

// TestShutdownScriptTime tests that shutdown scripts can run for around two minutes.
func TestShutdownScriptTime(t *testing.T) {
	outputPath := shutdownTimeOutputPath
	if runtime.GOOS == "windows" {
		outputPath = windowsShutdownTimeOutputPath
	}
	bytes, err := ioutil.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	if len(lines) < shutdownTime {
		t.Fatalf("shut down time is %d which is less than %d seconds.", len(lines), shutdownTime)
	}
	t.Logf("shut down time is %d", len(lines))
}
