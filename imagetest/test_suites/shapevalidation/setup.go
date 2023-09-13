package shapevalidation

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "shapevalidation"

type shape struct {
	name  string          // Full shape name
	cpu   int             // Expected number of vCPUs
	mem   uint64          // Expected memory in GB
	numa  uint8           // Expected number of vNUMA nodes
	disks []*compute.Disk // Disk configuration for created instances
	zone  string          // If set, force the VM to run in this zone
}

// Map of family name to the shape that should be tested in that family.
var x86shapes = map[string]*shape{
	"C3": {
		name:  "c3-highmem-176",
		cpu:   176,
		mem:   1408,
		numa:  4,
		disks: []*compute.Disk{{Name: "C3", Type: imagetest.PdBalanced, Zone: "us-east1-b"}},
		zone:  "us-east1-b",
	},
	"C3D": {
		name:  "c3d-highmem-360",
		cpu:   360,
		mem:   2880,
		numa:  2,
		disks: []*compute.Disk{{Name: "C3D", Type: imagetest.PdBalanced, Zone: "us-east4-c"}},
		zone:  "us-east4-c",
	},
	"E2": {
		name:  "e2-standard-32",
		cpu:   32,
		mem:   128,
		numa:  1,
		disks: []*compute.Disk{{Name: "E2", Type: imagetest.PdStandard}},
	},
	"N2": {
		name:  "n2-highmem-128",
		cpu:   128,
		mem:   864,
		numa:  2,
		disks: []*compute.Disk{{Name: "N2", Type: imagetest.PdStandard}},
	},
	"N2D": {
		name:  "n2d-standard-224",
		cpu:   224,
		mem:   896,
		numa:  2,
		disks: []*compute.Disk{{Name: "N2D", Type: imagetest.PdStandard}},
	},
	"T2D": {
		name:  "t2d-standard-60",
		cpu:   60,
		mem:   240,
		numa:  1,
		disks: []*compute.Disk{{Name: "T2D", Type: imagetest.PdStandard}},
	},
	"N1": {
		name:  "n1-highmem-96",
		cpu:   96,
		mem:   624,
		numa:  2,
		disks: []*compute.Disk{{Name: "N1", Type: imagetest.PdStandard}},
	},
}

var armshapes = map[string]*shape{
	"T2A": {
		name:  "t2a-standard-48",
		cpu:   48,
		mem:   192,
		numa:  1,
		disks: []*compute.Disk{{Name: "T2A", Type: imagetest.PdStandard, Zone: "us-central1-a"}},
		zone:  "us-central1-a",
	},
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if strings.Contains(t.Image, "arm") || strings.Contains(t.Image, "aarch") {
		return testFamily(t, armshapes)
	}
	return testFamily(t, x86shapes)
}

func testFamily(t *imagetest.TestWorkflow, families map[string]*shape) error {
	// This isn't because the test modifies project-level data, but because the
	// test uses so much capacity that we need to test images serially.
	t.LockProject()
	for family, shape := range families {
		vm, err := t.CreateTestVMMultipleDisks(shape.disks, map[string]string{})
		if err != nil {
			return err
		}
		if shape.zone != "" {
			vm.ForceZone(shape.zone)
		}
		vm.ForceMachineType(shape.name)
		vm.RunTests(fmt.Sprintf("(Test%sFamilyCpu)|(Test%sFamilyMem)|(Test%sFamilyNuma)", family, family, family))
	}
	return nil
}
