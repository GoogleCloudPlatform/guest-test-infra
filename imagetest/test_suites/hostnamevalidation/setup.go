package hostnamevalidation

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)


// Name is the name of the test package. It must match the directory name.
var Name = "hostnamevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// most of the tests are still linux specific for now, so skip most on windows.
	if utils.HasFeature(t.Image, "WINDOWS") {
		vm1, err := t.CreateTestVM("vm1")
		if err != nil {
			return err
		}
		vm1.RunTests("TestHostname")
		return nil
	} else {
		vm1, err := t.CreateTestVM("vm1")
		if err != nil {
			return err
		}
		vm1.RunTests("TestHostname|TestFQDN|TestHostKeysGeneratedOnce|TestHostsFile")

		vm2, err := t.CreateTestVM("vm2.custom.domain")
		if err != nil {
			return err
		}
		vm2.RunTests("TestCustomHostname")

		return nil
	}
}
