package image_validation

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/testmanager"
)

var Name = "image-validation"

func TestSetup(t *testmanager.TestWorkflow) error {
	return testmanager.SingleVMTest(t)
}
