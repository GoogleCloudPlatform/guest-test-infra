package windowsimagevalidation

import (
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "windowsimagevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if !strings.Contains(t.Image, "windows") {
		t.Skip("Test suite only valid on Windows images.")
	}
	_, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	return nil
}
