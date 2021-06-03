package ssh

import (
	"io/ioutil"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
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

	if err := utils.GenerateSSHKeyPair(user); err != nil {
		return err
	}
	vm.RunTests("TestSSHNonOsLogin")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	// add public key in metadata
	publicKey, err := ioutil.ReadFile("id_rsa.pub")
	if err != nil {
		return err
	}
	vm2.AddMetadata("ssh-keys", string(publicKey))
	vm2.RunTests("TestEmptyTest")
	return nil
}
