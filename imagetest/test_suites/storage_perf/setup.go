package storageperf

import (
	"fmt"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "storage_perf"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if bootdiskSize == HyperdiskSize {
		return fmt.Errorf("boot disk and mount disk must be different sizes for disk identification")
	}
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: vmName, Type: imagetest.PdBalanced, SizeGb: bootdiskSize},
		{Name: mountDiskName, Type: imagetest.HyperdiskExtreme, SizeGb: HyperdiskSize}})
	if err != nil {
		return err
	}
  vm.RunTests("TestReadIOPS")
	return nil
}
