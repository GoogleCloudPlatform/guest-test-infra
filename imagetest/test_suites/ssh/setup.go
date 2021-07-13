package ssh

import (
	"fmt"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "ssh"

const user = "test-user"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {

	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}

	vm.AddMetadata("block-project-ssh-keys", "true")

	publicKey, err := vm.AddTestUser()
	if err != nil {
		return err
	}
	vm.RunTests("TestSSH")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	// add public key in metadata
	vm2.AddMetadata("ssh-keys", fmt.Sprintf("%s:%s", user, string(publicKey)))
	vm2.RunTests("TestEmptyTest")
	return nil
}
