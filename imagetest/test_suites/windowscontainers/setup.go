package windowscontainers

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "windowscontainers"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if utils.HasFeature(t.Image, "WINDOWS") {
		_, err := t.CreateTestVM("vm")
		if err != nil {
			return err
		}
	}

	return nil
}
