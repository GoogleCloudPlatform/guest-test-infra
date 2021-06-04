package ssh

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "ssh"

const (
	user        = "test-user"
	osloginUser = "test-oslogin-user"
)

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
	vm.RunTests("TestSSH|TestGetentPasswdAllUsers|TestGetentPasswdOsLoginUser|TestGetentPasswdOsLoginUid")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.AddUser(user, publicKey)
	vm2.AddMetadata("enable-oslogin", "false")
	vm2.RunTests("TestEmptyTest")

	publicKeyOslogin, err := t.AddSSHKey(osloginUser)
	if err != nil {
		return err
	}
	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.AddUser(osloginUser, publicKeyOslogin)
	vm3.AddMetadata("enable-oslogin", "true")
	vm3.RunTests("TestEmptyTest")
	return nil
}
