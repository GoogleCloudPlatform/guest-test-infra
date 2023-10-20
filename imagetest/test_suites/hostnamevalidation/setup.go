package hostnamevalidation

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "hostnamevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
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
