package windowsimagevalidation

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "windowsimagevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if !utils.HasFeature(t.Image, "WINDOWS") {
		t.Skip("Test suite only valid on Windows images.")
	}
	_, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	return nil
}
