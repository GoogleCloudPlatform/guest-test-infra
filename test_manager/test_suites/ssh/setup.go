package ssh

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/testmanager"
)

var Name = "ssh"

func TestSetup(t *testmanager.TestWorkflow) error {
	vm1, _ := t.CreateTestVM("vm1")
	vm1.RunTests("TestVm1")
	vm2, _ := t.CreateTestVM("vm2")
	vm2.RunTests("TestVm2")
	return nil
}
