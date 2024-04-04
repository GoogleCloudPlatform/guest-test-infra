package disk

import (
	"strings"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

type blockdevNamingConfig struct {
	machineType string
	arch        string
}

var (
	// Name is the name of the test package. It must match the directory name.
	Name                = "disk"
	blockdevNamingCases = []blockdevNamingConfig{
		{
			machineType: "t2a-standard-1",
			arch:        "ARM64",
		},
		{
			machineType: "c3d-standard-4",
			arch:        "X86_64",
		},
	}
)

const (
	resizeDiskSize = 200
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	rebootInst := &daisy.Instance{}
	rebootInst.Metadata = map[string]string{imagetest.ShouldRebootDuringTest: "true"}
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "resize"}}, rebootInst)
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
	// Block device naming is an interaction between OS and hardware alone on windows, there is no guest-environment equivalent of udev rules for us to test.
	if !utils.HasFeature(t.Image, "WINDOWS") && utils.HasFeature(t.Image, "GVNIC") {
		for _, tc := range blockdevNamingCases {
			if tc.arch != t.Image.Architecture {
				continue
			}
			inst := &daisy.Instance{}
			inst.MachineType = tc.machineType
			series, _, _ := strings.Cut(tc.machineType, "-")
			inst.Name = "blockNaming" + strings.ToUpper(series)
			vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: inst.Name, Type: imagetest.PdBalanced}, {Name: "secondary", Type: imagetest.PdBalanced, SizeGb: 10}}, inst)
			if err != nil {
				return err
			}
			vm.RunTests("TestBlockDeviceNaming")
		}
	}
	return nil
}
