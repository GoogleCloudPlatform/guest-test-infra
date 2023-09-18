//go:build cit
// +build cit

package metadata

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	expectedStartupContent = "startup_success"
)

// TestStartupScriptFailed test that a script failed execute doesn't crash the vm.
func testStartupScriptFailedLinux() error {
	if _, err := utils.GetMetadataAttribute("startup-script"); err != nil {
		return fmt.Errorf("couldn't get startup-script from metadata, %v", err)
	}

	return nil
}

func testStartupScriptFailedWindows() error {
	if _, err := utils.GetMetadataAttribute("windows-startup-script-ps1"); err != nil {
		return fmt.Errorf("couldn't get windows-startup-script-ps1 from metadata, %v", err)
	}

	return nil
}

// TestDaemonScript test that daemon process started by startup script is still
// running in the VM after execution of startup script
func testDaemonScriptLinux() error {
	daemonOutputPath := "/daemon_out.txt"
	bytes, err := ioutil.ReadFile(daemonOutputPath)
	if err != nil {
		return fmt.Errorf("failed to read daemon script PID file: %v", err)
	}
	pid := strings.TrimSpace(string(bytes))
	cmd := exec.Command("ps", "-p", pid)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Daemon process not running: command \"ps -p %s\" failed: %v, output was: %s", pid, err, output)
	}

	return nil
}

func testDaemonScriptWindows() error {
	command := `Get-Process powershell`
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		return fmt.Errorf("Daemon process not found: %v", err)
	}

	job := strings.TrimSpace(output.Stdout)
	if !strings.Contains(job, "Running") {
		return fmt.Errorf("Daemon process found but not running: %s", job)
	}

	return nil
}

// TestStartupScripts verify that the standard metadata script could run successfully
// by checking the output content.
func TestStartupScripts(t *testing.T) {
	result, err := utils.GetMetadataGuestAttribute("testing/result")
	if err != nil {
		t.Fatalf("failed to read startup script result key: %v", err)
	}
	if result != expectedStartupContent {
		t.Fatalf(`startup script output expected "%s", got "%s".`, expectedStartupContent, result)
	}
}

func TestStartupScriptsFailed(t *testing.T) {
	if utils.IsWindows() {
		if err := testStartupScriptFailedWindows(); err != nil {
			t.Fatalf("Startup script failure test failed with error: %v", err)
		}
	} else {
		if err := testStartupScriptFailedLinux(); err != nil {
			t.Fatalf("Shutdown script failure test failed with error: %v", err)
		}
	}
}

func TestDaemonScripts(t *testing.T) {
	if utils.IsWindows() {
		if err := testDaemonScriptWindows(); err != nil {
			t.Fatalf("Daemon script test failed with error: %v", err)
		}
	} else {
		if err := testDaemonScriptLinux(); err != nil {
			t.Fatalf("Daemon script test failed with error: %v", err)
		}
	}
}
