//go:build cit
// +build cit

package metadata

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	expectedShutdownContent   = "Shutdown script success."
	shutdownOutputPathLinux   = "/shutdown_out.txt"
	shutdownOutputPathWindows = "C:\\shutdown_out.txt"
)

// The designed shutdown limit is 90s. Let's verify it's executed no less than 80s.
const shutdownTime = 80

// TestShutdownScript test the standard metadata script.
func testShutdownScriptLinux() error {
	bytes, err := ioutil.ReadFile(shutdownOutputPathLinux)
	if err != nil {
		return fmt.Errorf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != expectedShutdownContent {
		return fmt.Errorf(`shutdown script output expected "%s", got "%s"`, expectedShutdownContent, output)
	}

	return nil
}

func testShutdownScriptWindows() error {
	bytes, err := ioutil.ReadFile(shutdownOutputPathWindows)
	if err != nil {
		return fmt.Errorf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != expectedShutdownContent {
		return fmt.Errorf(`shutdown script output expected "%s", got "%s"`, expectedShutdownContent, output)
	}

	return nil
}

// TestShutdownScriptFailed test that a script failed execute doesn't crash the vm.
func testShutdownScriptFailedLinux() error {
	if _, err := utils.GetMetadataAttribute("shutdown-script"); err != nil {
		return fmt.Errorf("couldn't get shutdown-script from metadata")
	}

	return nil

}

func testShutdownScriptFailedWindows() error {
	if _, err := utils.GetMetadataAttribute("shutdown-script-ps1"); err != nil {
		return fmt.Errorf("couldn't get shutdown-script from metadata")
	}

	return nil

}

// TestShutdownUrlScript test that URL scripts work correctly.
func testShutdownUrlScriptLinux() error {
	bytes, err := ioutil.ReadFile(shutdownOutputPathLinux)
	if err != nil {
		return fmt.Errorf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != expectedShutdownContent {
		return fmt.Errorf(`shutdown script output expected "%s", got "%s"`, expectedShutdownContent, output)
	}

	return nil

}

func testShutdownUrlScriptWindows() error {
	bytes, err := ioutil.ReadFile(shutdownOutputPathWindows)
	if err != nil {
		return fmt.Errorf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != expectedShutdownContent {
		return fmt.Errorf(`shutdown script output expected "%s", got "%s"`, expectedShutdownContent, output)
	}

	return nil

}

// TestShutdownScriptTime test that shutdown scripts can run for around two minutes
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

func testShutdownScriptTimeWindows() error {
	bytes, err := ioutil.ReadFile("C:\\shutdown.txt")
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

func TestShutdownScripts(t *testing.T) {
	if utils.IsWindows() {
		if err := testShutdownScriptWindows(); err != nil {
			t.Fatalf("Shutdown script test failed with error: %v", err)
		}
	} else {
		if err := testShutdownScriptLinux(); err != nil {
			t.Fatalf("Shutdown script test failed with error: %v", err)
		}
	}
}

func TestShutdownScriptsFailed(t *testing.T) {
	if utils.IsWindows() {
		if err := testShutdownScriptFailedWindows(); err != nil {
			t.Fatalf("Shutdown script failure test failed with error: %v", err)
		}
	} else {
		if err := testShutdownScriptFailedLinux(); err != nil {
			t.Fatalf("Shutdown script failure test failed with error: %v", err)
		}
	}
}

func TestShutdownUrlScripts(t *testing.T) {
	if utils.IsWindows() {
		if err := testShutdownUrlScriptWindows(); err != nil {
			t.Fatalf("Shutdown script URL test failed with error: %v", err)
		}
	} else {
		if err := testShutdownUrlScriptLinux(); err != nil {
			t.Fatalf("Shutdown script URL test failed with error: %v", err)
		}
	}
}
