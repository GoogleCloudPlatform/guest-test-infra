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
	vm, err := t.CreateTestVM("client")
	if err != nil {
		return err
	}
	vm.AddMetadata("block-project-ssh-keys", "true")
	vm.AddMetadata("enable-guest-attributes", "true")
	vm.AddMetadata("enable-windows-ssh", "true")
	vm.AddMetadata("sysprep-specialize-script-cmd", "googet -noconfirm=true install google-compute-engine-ssh")
	vm.RunTests("TestSSHInstanceKey|TestHostKeysAreUnique|TestMatchingKeysInGuestAttributes")

	vm2, err := t.CreateTestVM("server")
	if err != nil {
		return err
	}
	vm2.AddUser(user, publicKey)
	vm2.AddMetadata("enable-guest-attributes", "true")
	vm2.AddMetadata("enable-oslogin", "false")
	vm2.AddMetadata("enable-windows-ssh", "true")
	vm2.AddMetadata("sysprep-specialize-script-cmd", "googet -noconfirm=true install google-compute-engine-ssh")
	vm2.RunTests("TestEmptyTest")

	vm3, err := t.CreateTestVM("hostkeysafteragentrestart")
	if err != nil {
		return err
	}
	vm3.RunTests("TestHostKeysNotOverrideAfterAgentRestart")
	return nil
}
