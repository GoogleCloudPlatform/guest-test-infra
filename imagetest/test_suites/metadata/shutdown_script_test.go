// +build cit

package metadata

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const shutdownTime = 110 // about 2 minutes

// TestShutdownScript test the standard metadata script.
func TestShutdownScript(t *testing.T) {
	// second boot
	bytes, err := ioutil.ReadFile(shutdownOutputPath)
	if err != nil {
		t.Fatalf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != shutdownContent {
		t.Fatalf(`shutdown script output expect "%s", but actually "%s"`, shutdownContent, output)
	}
}

// TestShutdownScriptFailedNotCrashVM test that a script failed execute doesn't
// crash the vm.
func TestShutdownScriptFailedNotCrashVM(t *testing.T) {
	// second boot
	if _, err := utils.GetMetadataAttribute("shutdown-script"); err != nil {
		t.Fatalf("couldn't get shutdown-script from metadata")
	}
}

// TestShutdownUrlScript test that URL scripts work correctly.
func TestShutdownUrlScript(t *testing.T) {
	// second boot
	bytes, err := ioutil.ReadFile(shutdownOutputPath)
	if err != nil {
		t.Fatalf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != shutdownContent {
		t.Fatalf(`shutdown script output expect "%s", but actually "%s"`, shutdownContent, output)
	}
}

// TestShutdownScriptTime test that shutdown scripts can run for around two minutes
func TestShutdownScriptTime(t *testing.T) {
	// second boot
	bytes, err := ioutil.ReadFile("/shutdown.txt")
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	if len(lines) < shutdownTime {
		t.Fatalf("shut down time less than %d seconds.", shutdownTime)
	}
}
