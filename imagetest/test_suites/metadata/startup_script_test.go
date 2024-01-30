//go:build cit
// +build cit

package metadata

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	expectedStartupContent = "startup_success"
)

// TestStartupScriptFailedLinux tests that a script failed execute doesn't crash the vm.
func testStartupScriptFailedLinux(t *testing.T) error {
	if _, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "startup-script"); err != nil {
		return fmt.Errorf("couldn't get startup-script from metadata, %v", err)
	}

	return nil
}

// TestStartupScriptFailedWindows tests that a script failed execute doesn't crash the vm.
func testStartupScriptFailedWindows(t *testing.T) error {
	if _, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "windows-startup-script-ps1"); err != nil {
		return fmt.Errorf("couldn't get windows-startup-script-ps1 from metadata, %v", err)
	}

	return nil
}

// TestDaemonScriptLinux tests that daemon process started by startup script is still
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

// TestDaemonScriptWindows tests that background cmd process started by startup script is still
// running in the VM after execution of startup script
func testDaemonScriptWindows() error {
	command := `Get-Process cmd`
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		return fmt.Errorf("Daemon process not found: %v", err)
	}

	job := strings.TrimSpace(output.Stdout)
	if !strings.Contains(job, "cmd") {
		return fmt.Errorf("Daemon process not running. Output of Get-Process: %s", job)
	}

	return nil
}

// TestStartupScripts verifies that the standard metadata script could run successfully
// by checking the output content of the Startup script. It also checks that
// the script does not run after a reinstall/upgrade of guest agent.
func TestStartupScripts(t *testing.T) {
	result, err := utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "result")
	if err != nil {
		t.Fatalf("failed to read startup script result key: %v", err)
	}
	if result != expectedStartupContent {
		t.Fatalf(`startup script output expected "%s", got "%s".`, expectedStartupContent, result)
	}
	err = utils.PutMetadata(utils.Context(t), path.Join("instance", "guest-attributes", "testing", "result"), "")
	if err != nil {
		t.Fatalf("failed to clear startup script result: %s", err)
	}
	if utils.IsWindows() {
		if err := reinstallPackage("google-compute-engine-windows"); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := reinstallPackage("google-guest-agent"); err != nil {
			t.Fatal(err)
		}
	}
	result, err = utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "result")
	if err != nil {
		t.Fatalf("failed to read startup script result key: %v", err)
	}
	if result == expectedStartupContent {
		t.Errorf("startup script reexected after a reinstall of guest agent")
	}
}

// Determine if the OS is Windows or Linux and run the appropriate failure test.
func TestStartupScriptsFailed(t *testing.T) {
	if utils.IsWindows() {
		if err := testStartupScriptFailedWindows(t); err != nil {
			t.Fatalf("Startup script failure test failed with error: %v", err)
		}
	} else {
		if err := testStartupScriptFailedLinux(t); err != nil {
			t.Fatalf("Shutdown script failure test failed with error: %v", err)
		}
	}
}

// Determine if the OS is Windows or Linux and run the appropriate daemon test.
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
