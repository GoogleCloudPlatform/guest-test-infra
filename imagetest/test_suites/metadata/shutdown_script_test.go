// go:build cit

package metadata

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// The designed shutdown limit is 90s. Let's verify it's executed no less than 80s.
const shutdownTime = 80

// TestShutdownScript test the standard metadata script.
func TestShutdownScript(t *testing.T) {
	bytes, err := ioutil.ReadFile(shutdownOutputPath)
	if err != nil {
		t.Fatalf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != shutdownContent {
		t.Fatalf(`shutdown script output expect "%s", but actually "%s"`, shutdownContent, output)
	}
}

// TestShutdownScriptFailed test that a script failed execute doesn't crash the vm.
func TestShutdownScriptFailed(t *testing.T) {
	if _, err := utils.GetMetadataAttribute("shutdown-script"); err != nil {
		t.Fatalf("couldn't get shutdown-script from metadata")
	}
}

// TestShutdownUrlScript test that URL scripts work correctly.
func TestShutdownUrlScript(t *testing.T) {
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
	bytes, err := ioutil.ReadFile("/shutdown.txt")
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	if len(lines) < shutdownTime {
		t.Fatalf("shut down time is %d which is less than %d seconds.", len(lines), shutdownTime)
	}
	t.Logf("shut down time is %d", len(lines))
}
