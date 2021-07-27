package metadata

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	_, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	return nil
}
