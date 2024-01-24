//go:build cit
// +build cit

package shapevalidation

import (
	"strconv"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestMem(t *testing.T) {
	expectedMemory, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "expected_memory")
	if err != nil {
		t.Fatalf("could not get expected memory from metadata: %v", err)
	}
	emem, err := strconv.ParseUint(expectedMemory, 10, 64)
	if err != nil {
		t.Fatalf("could not parse uint64 from %s", expectedMemory)
	}
	mem, err := memTotal()
	if err != nil {
		t.Fatal(err)
	}
	if mem < emem {
		t.Errorf("got %d GB memory, want at least %d GB", mem, emem)
	}
}

func TestCpu(t *testing.T) {
	expectedCpu, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "expected_cpu")
	if err != nil {
		t.Fatalf("could not get expected cpu count from metadata: %v", err)
	}
	ecpu, err := strconv.Atoi(expectedCpu)
	if err != nil {
		t.Fatalf("could not parse int from %s", expectedCpu)
	}
	cpu, err := numCpus()
	if err != nil {
		t.Fatal(err)
	}
	if cpu != ecpu {
		t.Errorf("got %d CPUs want %d", cpu, ecpu)
	}
}

func TestNuma(t *testing.T) {
	expectedNuma, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "expected_numa")
	if err != nil {
		t.Fatalf("could not get expected numa node count from metadata: %v", err)
	}
	enuma, err := strconv.ParseUint(expectedNuma, 10, 8)
	if err != nil {
		t.Fatalf("could not parse uint8 from %s", expectedNuma)
	}
	numa, err := numNumaNodes()
	if err != nil {
		t.Fatal(err)
	}
	if !reliableNuma() {
		t.Skip("numa node counts are not reliable on this VM/OS combination")
	}
	if numa != uint8(enuma) {
		t.Errorf("got %d numa nodes, want %d", numa, uint8(enuma))
	}
}
