// +build cit

package metadata

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestStartupScript verify that the standard metadata script could run successfully
// by checking the output content.
func TestStartupScript(t *testing.T) {
	bytes, err := ioutil.ReadFile(startupOutputPath)
	if err != nil {
		t.Fatalf("failed to read startup script output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != startupContent {
		t.Fatalf(`startup script output expect "%s", but actually "%s"`, startupOutputPath, output)
	}
}

// TestStartupScriptFailed test that a script failed execute doesn't crash the vm.
func TestStartupScriptFailed(t *testing.T) {
	if _, err := utils.GetMetadataAttribute("startup-script"); err != nil {
		t.Fatalf("couldn't get startup-script from metadata, %v", err)
	}
}
