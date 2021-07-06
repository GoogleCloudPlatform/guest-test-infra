package ssh

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "ssh"

const FRAMEWORK_USERNAME = "sa_115023165530670019835"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.AddMetadata("ssh_user_name", FRAMEWORK_USERNAME)
	vm.AddMetadata("enable-oslogin", "true")

	//vm3, err := t.CreateTestVM("vm3")
	//if err != nil {
	//	return err
	//}
	//vm3.AddMetadata("ssh_user_name", FRAMEWORK_USERNAME)
	//vm3.AddMetadata("enable-oslogin", "true")
	//vm3.AddMetadata("enable-oslogin-2fa", "true")
	//vm3.RunTests("")
	//
	//vm4, err := t.CreateTestVM("vm4")
	//if err != nil {
	//	return err
	//}
	//vm4.AddMetadata("ssh_user_name", FRAMEWORK_USERNAME)
	//vm4.AddMetadata("enable-oslogin", "true")
	//vm4.AddMetadata("enable-oslogin-2fa", "true")
	//vm4.AddMetadata("command-count", "0")
	//vm4.RunTests("")

	return err
}
