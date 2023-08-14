package storageperf

import (
	"fmt"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "storage_perf"

const (
	vmName = "vm"
	// HyperdiskSize is used to determine which partition is the mounted hyperdisk.
	HyperdiskSize = 100
	bootdiskSize  = 10
	mountDiskName = "hyperdisk"
	// TODO: Set up constants for compute.Disk.ProvisionedIOPS int64, and compute.Disk.ProvisionedThrougput int64, then set these fields in appendCreateDisksStep
)

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
	vm.UseGVNIC()
	return nil
}
