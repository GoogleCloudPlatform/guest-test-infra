package winrm

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "winrm"

const user = "test-user"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if !utils.HasFeature(t.Image, "WINDOWS") {
		return nil
	}
	passwd := utils.ValidWindowsPassword(15)

	vm, err := t.CreateTestVM("client")
	if err != nil {
		return err
	}
	vm.AddMetadata("winrm-passwd", passwd)
	vm.RunTests("TestWinrmConnection")

	vm2, err := t.CreateTestVM("server")
	if err != nil {
		return err
	}
	vm2.AddMetadata("winrm-passwd", passwd)
	vm2.RunTests("TestWaitForWinrmConnection")

	return nil
}
