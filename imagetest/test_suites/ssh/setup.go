package ssh

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "ssh"

const user = "test-user"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// adds the private key to the t.wf.Sources
	publicKey, err := t.AddSSHKey(user)
	if err != nil {
		return err
	}
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.AddMetadata("block-project-ssh-keys", "true")
	vm.RunTests("TestSSH")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.AddUser(user, publicKey)
	vm2.AddMetadata("enable-oslogin", "false")
	vm2.RunTests("TestEmptyTest")
	return nil
}
