package oslogin

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "oslogin"

const (
	computeScope  = "https://www.googleapis.com/auth/compute"
	platformScope = "https://www.googleapis.com/auth/cloud-platform"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	defaultVM, err := t.CreateTestVM("default")
	if err != nil {
		return err
	}
	defaultVM.AddScope(computeScope)
	defaultVM.AddMetadata("enable-oslogin", "true")
	defaultVM.RunTests("TestOsLoginEnabled|TestGetentPasswd|TestAgent")

	ssh, err := t.CreateTestVM("ssh")
	if err != nil {
		return err
	}
	ssh.AddScope(platformScope)
	ssh.AddMetadata("enable-oslogin", "false")
	ssh.RunTests("TestOsLoginDisabled|TestSSH|TestAdminSSH")

	twofa, err := t.CreateTestVM("twofa")
	if err != nil {
		return err
	}
	twofa.AddScope(computeScope)
	twofa.AddMetadata("enable-oslogin", "true")
	twofa.AddMetadata("enable-oslogin-2fa", "true")
	twofa.RunTests("TestAgent")
	return nil
}
