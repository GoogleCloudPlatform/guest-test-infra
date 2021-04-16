package imageboot

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "image-boot"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	rebootVM, err := t.CreateTestVM("reboot-test")
	if err != nil {
		return err
	}
	return rebootVM.Reboot()
}
