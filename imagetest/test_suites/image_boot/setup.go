package image_boot

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

var Name = "image-boot"

func TestSetup(t *imagetest.TestWorkflow) error {
	rebootVm, err := t.CreateTestVM("reboot-test")
	if err != nil {
		return err
	}
	rebootVm.Reboot()
	return nil
}
