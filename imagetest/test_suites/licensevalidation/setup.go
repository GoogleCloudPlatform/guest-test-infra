package licensevalidation

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "licensevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm1, err := t.CreateTestVM("vm1")
	if err != nil {
		return err
	}
	if utils.HasFeature(t.Image, "WINDOWS") {
		vm1.RunTests("TestWindowsActivationStatus")
	} else {
		vm1.RunTests("TestArePackagesLegal")
	}

	return nil
}
