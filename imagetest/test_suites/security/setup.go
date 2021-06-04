package security

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "security"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	_, err := t.CreateTestVM("vm")
	return err
}
