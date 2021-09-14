package preinstalled

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "preinstalled"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	_, err := t.CreateTestVM("vm1")
	if err != nil {
		return err
	}
	return nil
}
