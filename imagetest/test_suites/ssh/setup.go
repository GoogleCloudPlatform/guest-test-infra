package ssh

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "ssh"

const (
	testUser = "test-user"
	testUserLegacy = "test-user-legacy"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// adds the private key to the t.wf.Sources
	publicKey, err := t.AddSSHKey(testUser)
	if err != nil {
		return err
	}
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.AddMetadata("block-project-ssh-keys", "true")
	vm.RunTests("TestSSHInstanceKey")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.AddUser(testUser, publicKey)
	vm2.RunTests("TestEmptyTest")

	// adds the private key to the t.wf.Sources
	publicKey, err = t.AddSSHKey(testUserLegacy)
	if err != nil {
		return err
	}
	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.AddMetadata("block-project-ssh-keys", "true")
	vm3.RunTests("TestSSHInstanceKey")

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	vm4.AddUserLegacyKey(testUserLegacy, publicKey)
	vm4.RunTests("TestEmptyTest")
	return nil
}
