package imagevalidation

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "image_validation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm1, err := t.CreateTestVM("vm1")
	if err != nil {
		return err
	}
	vm1.RunTests("TestHostname|TestFQDN|TestHostKeysGeneratedOnce|TestArePackagesLegal|TestLinuxLicense")

	vm2, err := t.CreateTestVM("vm2.custom.domain")
	if err != nil {
		return err
	}
	vm2.RunTests("TestCustomHostname")

	return nil
}
