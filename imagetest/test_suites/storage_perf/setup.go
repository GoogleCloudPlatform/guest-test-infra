package storageperf

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "storage_perf"

const (
	vmName = "vm"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// mount the hyperdisk as a startup script
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: vmName},
		{Name: "pdextreme", Type: imagetest.PdExtreme, SizeGb: 100}})
	if err != nil {
		return err
	}
	vm.UseGVNIC()
	return nil
}
