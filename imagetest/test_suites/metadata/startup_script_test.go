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
	expectedStartupContent = "Startup script success."
)

// TestStartupScript verify that the standard metadata script could run successfully
// by checking the output content.
func testStartupScriptLinux() error {
	startupOutputPath := "/startup_out.txt"
	bytes, err := ioutil.ReadFile(startupOutputPath)
	if err != nil {
		return fmt.Errorf("failed to read startup script output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != expectedStartupContent {
		return fmt.Errorf(`startup script output expected "%s", got "%s"`, expectedStartupContent, output)
	}

	return nil
}

func testStartupScriptWindows() error {
	startupOutputPath := "C:\\startup_out.txt"
	bytes, err := ioutil.ReadFile(startupOutputPath)
	if err != nil {
		return fmt.Errorf("failed to read startup script output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != expectedStartupContent {
		return fmt.Errorf(`startup script output expected "%s", got "%s"`, expectedStartupContent, output)
	}

	return nil
}

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
func testDaemonScriptLinux(t *testing.T) {
	daemonOutputPath := "/daemon_out.txt"
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

func TestStartupScripts(t *testing.T) {
	if utils.IsWindows() {
		if err := testStartupScriptWindows(); err != nil {
			t.Fatalf("Startup script test failed with error: %v", err)
		}
	} else {
		if err := testStartupScriptLinux(); err != nil {
			t.Fatalf("Startup script test failed with error: %v", err)
		}
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
