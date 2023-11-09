package disk

import (
	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "disk"

const (
	vmName         = "vm"
	resizeDiskSize = 200
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	rebootInst := &daisy.Instance{}
	rebootInst.Metadata = map[string]string{imagetest.ShouldRebootDuringTest: "true"}
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: vmName}}, rebootInst)
	if err != nil {
		return err
	}
	// TODO:currently the Resize and Reboot disk test is only written to run on linux
	if !utils.HasFeature(t.Image, "WINDOWS") {
		if err = vm.ResizeDiskAndReboot(resizeDiskSize); err != nil {
			return err
		}
	}
	vm.RunTests("TestDiskReadWrite|TestDiskResize")
	return nil
}
