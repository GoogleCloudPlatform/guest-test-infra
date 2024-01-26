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
		vm2, err := t.CreateTestVM("vm2")
		if err != nil {
			return err
		}
		vm2.RunTests("TestGooGetInstalled|TestGooGetAvailable|TestSigned|TestRemoveInstall" +
			"|TestPackagesInstalled|TestPackagesAvailable|TestPackagesSigned")
		vm3, err := t.CreateTestVM("vm3")
		if err != nil {
			return err
		}
		vm3.RunTests("TestRepoManagement")
		vm4, err := t.CreateTestVM("vm4")
		if err != nil {
			return err
		}
		vm4.RunTests("TestNetworkDriverLoaded|TestDriversInstalled|TestDriversRemoved")
		// the former windows_image_validation test suite tests are run by this VM.
		// It may make sense to move some of these tests to other suites in the future.
		vm5, err := t.CreateTestVM("vm5")
		if err != nil {
			return err
		}
		vm5.RunTests("TestAutoUpdateEnabled|TestNetworkConnecton|TestEmsEnabled" +
			"|TestTimeZoneUTC|TestPowershellVersion|TestStartExe|TestDotNETVersion" +
			"|TestServicesState|TestWindowsEdition|TestWindowsCore")
		sysprepvm, err := t.CreateTestVM("gcesysprep")
		if err != nil {
			return err
		}
		sysprepvm.RunTests("TestGCESysprep")
	}
	return nil
}
