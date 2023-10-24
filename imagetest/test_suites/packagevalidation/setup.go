package packagevalidation

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "packagevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm1, err := t.CreateTestVM("vm1")
	if err != nil {
		return err
	}
	vm1.RunTests("TestStandardPrograms|TestGuestPackages|TestNTP")

	// as part of the migration of the windows test suite, these vms
	// are only used to run windows tests. The tests themselves
	// have components which need to be run on different vms.
	if utils.HasFeature(t.Image, "WINDOWS") {
		vm, err := t.CreateTestVM("vm2")
		if err != nil {
			return err
		}
		vm.RunTests("TestGooGetInstalled|TestGooGetAvailable|TestSigned|TestRemoveInstall" +
			"|TestPackagesInstalled|TestPackagesAvailable|TestPackagesSigned")
		vm2, err := t.CreateTestVM("vm3")
		if err != nil {
			return err
		}
		vm2.RunTests("TestRepoManagement")
		vm3, err := t.CreateTestVM("vm4")
		if err != nil {
			return err
		}
		vm3.RunTests("TestNetworkDriverLoaded|TestDriversInstalled|TestDriversRemoved")
	}
	return nil
}
