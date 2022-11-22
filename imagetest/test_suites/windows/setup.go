package windows

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "windows"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestGooGetInstalled|TestGooGetAvailable|TestSigned|TestRemoveInstall" +
		"|TestPackagesInstalled|TestPackagesAvailable|TestPackagesSigned")
	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.RunTests("TestRepoManagement")
	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.RunTests("TestNetworkDriverLoaded|TestDriversInstalled|TestDriversRemoved")
	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	vm4.ChangeNicTypeToGVNIC()
	if err := vm4.Reboot(); err != nil {
		return err
	}
	vm4.RunTests("TestGVNIC")
	return nil
}
