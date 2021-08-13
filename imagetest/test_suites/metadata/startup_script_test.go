// +build cit

package metadata

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestStartupScript test the standard metadata script.
func TestStartupScript(t *testing.T) {
	bytes, err := ioutil.ReadFile(startupOutputPath)
	if err != nil {
		t.Fatalf("failed to read shutdown output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != startupContent {
		t.Fatalf(`shutdown script output expect "%s", but actually "%s"`, startupOutputPath, output)
	}
}

// TestStartupScriptFailed test that a script failed execute doesn't crash the vm.
func TestStartupScriptFailed(t *testing.T) {
	if _, err := utils.GetMetadataAttribute("startup-script"); err != nil {
		t.Fatalf("couldn't get startup-script from metadata, %v", err)
	}
}
