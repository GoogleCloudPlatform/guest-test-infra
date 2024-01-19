//go:build cit
// +build cit

package metadata

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	expectedShutdownContent = "shutdown_success"
	// The designed shutdown limit is 90s. Let's verify it's executed no less than 80s.
	shutdownTime = 80
)

// TestShutdownScriptFailedLinux tests that a script failed execute doesn't crash the vm.
func testShutdownScriptFailedLinux(t *testing.T) error {
	if _, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "shutdown-script"); err != nil {
		return fmt.Errorf("couldn't get shutdown-script from metadata")
	}

	return nil

}

// TestShutdownScriptFailedWindows tests that a script failed execute doesn't crash the vm.
func testShutdownScriptFailedWindows(t *testing.T) error {
	if _, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "windows-shutdown-script-ps1"); err != nil {
		return fmt.Errorf("couldn't get windows-shutdown-script-ps1 from metadata")
	}

	return nil

}

// TestShutdownScriptTimeLinux tests that shutdown scripts can run for around two minutes.
func testShutdownScriptTimeLinux() error {
	bytes, err := ioutil.ReadFile("/shutdown.txt")
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	if len(lines) < shutdownTime {
		return fmt.Errorf("shut down time is %d which is less than %d seconds.", len(lines), shutdownTime)
	}
	fmt.Sprintf("shut down time is %d", len(lines))

	return nil

}

// TestShutdownScriptTimeWindows tests that shutdown scripts can run for around two minutes.
func testShutdownScriptTimeWindows() error {
	bytes, err := ioutil.ReadFile("C:\\shutdown_out.txt")
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	if len(lines) < shutdownTime {
		return fmt.Errorf("shut down time is %d which is less than %d seconds.", len(lines), shutdownTime)
	}
	fmt.Sprintf("shut down time is %d", len(lines))

	return nil

}

// TestShutdownScripts verifies that the standard metadata script could run successfully
// by checking the output content of the Shutdown script. It also checks that
// shutdown scripts don't run on reinstall or upgrade of the guest-agent.
func TestShutdownScripts(t *testing.T) {
	result, err := utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "result")
	if err != nil {
		t.Fatalf("failed to read shutdown script result key: %v", err)
	}
	if result != expectedShutdownContent {
		t.Errorf(`shutdown script output expected "%s", got "%s".`, expectedShutdownContent, result)
	}
	err = utils.PutMetadata(utils.Context(t), path.Join("instance", "guest-attributes", "testing", "result"), "")
	if err != nil {
		t.Fatalf("failed to clear shutdown script result: %s", err)
	}
	if err := reinstallPackage("google-guest-agent"); err != nil {
		t.Fatal(err)
	}
	result, err = utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "result")
	if err != nil {
		t.Fatalf("failed to read shutdown script result key: %v", err)
	}
	if result == expectedShutdownContent {
		t.Errorf("shutdown script executed after a reinstall of guest agent")
	}
}

// Determine if the OS is Windows or Linux and run the appropriate failure test.
func TestShutdownScriptsFailed(t *testing.T) {
	if utils.IsWindows() {
		if err := testShutdownScriptFailedWindows(t); err != nil {
			t.Fatalf("Shutdown script failure test failed with error: %v", err)
		}
	} else {
		if err := testShutdownScriptFailedLinux(t); err != nil {
			t.Fatalf("Shutdown script failure test failed with error: %v", err)
		}
	}
}

// Determine if the OS is Windows or Linux and run the appropriate daemon test.
func TestShutdownURLScripts(t *testing.T) {
	result, err := utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "result")
	if err != nil {
		t.Fatalf("failed to read shutdown script result key: %v", err)
	}
	if result != expectedShutdownContent {
		t.Fatalf(`shutdown script output expected "%s", got "%s".`, expectedShutdownContent, result)
	}
}

// Determine if the OS is Windows or Linux and run the appropriate shutdown time test.
func TestShutDownScriptTime(t *testing.T) {
	if utils.IsWindows() {
		if err := testShutdownScriptTimeWindows(); err != nil {
			t.Fatalf("Shutdown script time test failed with error: %v", err)
		}
	} else {
		if err := testShutdownScriptTimeLinux(); err != nil {
			t.Fatalf("Shutdown script time test failed with error: %v", err)
		}
	}
}
