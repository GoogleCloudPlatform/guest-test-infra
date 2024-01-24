package shapevalidation

import (
	"flag"
	"fmt"
	"regexp"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "shapevalidation"

var testFilter = flag.String("shapevalidation_test_filter", ".*", "regexp filter for shapevalidation test cases, only cases with a matching family name will be run")

type shape struct {
	name string // Full shape name
	// TODO use the compute API to fetch cpu and memory amounts instead of
	// hardcoding a list.
	cpu             int                   // Expected number of vCPUs
	mem             uint64                // Expected memory in GB
	numa            uint8                 // Expected number of vNUMA nodes
	disks           []*compute.Disk       // Disk configuration for created instances
	zone            string                // If set, force the VM to run in this zone
	requireFeatures []string              // Features necessary for testing this shape
	excludeFeatures []string              // Features which prevent testing this shape
	exceptions      []*regexp.Regexp      // Regexp matches for image names to exempt
	quota           *daisy.QuotaAvailable // Quota necessary to run the test
}

// Map of test name to the shape that should be tested.
var x86shapes = map[string]*shape{
	"C3": {
		name:            "c3-highmem-176",
		cpu:             176,
		mem:             1408,
		numa:            4,
		disks:           []*compute.Disk{{Name: "C3", Type: imagetest.PdBalanced, Zone: "us-east1-b"}},
		zone:            "us-east1-b",
		requireFeatures: []string{"GVNIC"},
		quota:           &daisy.QuotaAvailable{Metric: "C3_CPUS", Units: 176, Region: "us-east1"},
	},
	"C3D": {
		name:            "c3d-highmem-360",
		cpu:             360,
		mem:             2880,
		numa:            2,
		disks:           []*compute.Disk{{Name: "C3D", Type: imagetest.PdBalanced, Zone: "us-east4-c"}},
		zone:            "us-east4-c",
		requireFeatures: []string{"GVNIC"},
		excludeFeatures: []string{"WINDOWS"},
		exceptions:      []*regexp.Regexp{regexp.MustCompile("(rhel|centos|almalinux|rocky-linux)-7")},
		quota:           &daisy.QuotaAvailable{Metric: "CPUS", Units: 176, Region: "us-east4"}, // No public C3D metric yet
	},
	"E2": {
		name:  "e2-standard-32",
		cpu:   32,
		mem:   128,
		numa:  1,
		disks: []*compute.Disk{{Name: "E2", Type: imagetest.PdStandard}},
		quota: &daisy.QuotaAvailable{Metric: "E2_CPUS", Units: 32},
	},
	"C4": {
		name:            "c4-highmem-192",
		cpu:             192,
		mem:             1488,
		numa:            2,
		zone:            "us-east5-b",
		disks:           []*compute.Disk{{Name: "C4-192", Type: imagetest.HyperdiskBalanced, Zone: "us-east5-b"}},
		quota:           &daisy.QuotaAvailable{Metric: "CPUS", Units: 192, Region: "us-east5"},
		requireFeatures: []string{"GVNIC"},
	},
	"N2": {
		name:  "n2-highmem-128",
		cpu:   128,
		mem:   864,
		numa:  2,
		disks: []*compute.Disk{{Name: "N2", Type: imagetest.PdStandard}},
		quota: &daisy.QuotaAvailable{Metric: "N2_CPUS", Units: 128},
	},
	"N2D": {
		name:  "n2d-standard-224",
		cpu:   224,
		mem:   896,
		numa:  2,
		disks: []*compute.Disk{{Name: "N2D", Type: imagetest.PdStandard}},
		quota: &daisy.QuotaAvailable{Metric: "N2D_CPUS", Units: 224},
	},
	"T2D": {
		name:  "t2d-standard-60",
		cpu:   60,
		mem:   240,
		numa:  1,
		disks: []*compute.Disk{{Name: "T2D", Type: imagetest.PdStandard}},
		quota: &daisy.QuotaAvailable{Metric: "T2D_CPUS", Units: 60},
	},
	"N1": {
		name:  "n1-highmem-96",
		cpu:   96,
		mem:   624,
		numa:  2,
		disks: []*compute.Disk{{Name: "N1", Type: imagetest.PdStandard}},
		quota: &daisy.QuotaAvailable{Metric: "CPUS", Units: 96},
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
		quota: &daisy.QuotaAvailable{Metric: "T2A_CPUS", Units: 48, Region: "us-central1"},
	},
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if t.Image.Architecture == "ARM64" {
		return testFamily(t, armshapes)
	}
	return testFamily(t, x86shapes)
}

func testFamily(t *imagetest.TestWorkflow, families map[string]*shape) error {
	filter, err := regexp.Compile(*testFilter)
	if err != nil {
		return fmt.Errorf("invalid shapevalidation test filter: %v", err)
	}
	// This isn't because the test modifies project-level data, but because the
	// test uses so much capacity that we need to test images serially.
	t.LockProject()
Familyloop:
	for family, shape := range families {
		if !filter.MatchString(family) {
			continue
		}
		for _, feat := range shape.requireFeatures {
			if !utils.HasFeature(t.Image, feat) {
				continue Familyloop
			}
		}
		for _, feat := range shape.excludeFeatures {
			if utils.HasFeature(t.Image, feat) {
				continue Familyloop
			}
		}
		for _, r := range shape.exceptions {
			if r.MatchString(t.Image.Name) {
				continue Familyloop
			}
		}
		if shape.quota != nil {
			if err := t.WaitForVMQuota(shape.quota); err != nil {
				return err
			}
		}
		vm, err := t.CreateTestVMMultipleDisks(shape.disks, nil)
		if err != nil {
			return err
		}
		if shape.zone != "" {
			vm.ForceZone(shape.zone)
		}
		vm.ForceMachineType(shape.name)
		vm.AddMetadata("expected_memory", fmt.Sprintf("%d", shape.mem))
		vm.AddMetadata("expected_cpu", fmt.Sprintf("%d", shape.cpu))
		vm.AddMetadata("expected_numa", fmt.Sprintf("%d", shape.numa))
		vm.RunTests("(TestCpu)|(TestMem)|(TestNuma)")
	}
	return nil
}
