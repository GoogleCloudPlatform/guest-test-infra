//go:build cit
// +build cit

package shapevalidation

import (
	"testing"
)

func testMem(t *testing.T, s *shape) {
	mem, err := memTotal()
	if err != nil {
		t.Fatal(err)
	}
	if mem < s.mem {
		t.Errorf("got %d GB memory, want at least %d GB", mem, s.mem)
	}
}

func testCpu(t *testing.T, s *shape) {
	cpu, err := numCpus()
	if err != nil {
		t.Fatal(err)
	}
	if cpu != s.cpu {
		t.Errorf("got %d CPUs want %d", cpu, s.cpu)
	}
}

func testNuma(t *testing.T, s *shape) {
	numa, err := numNumaNodes()
	if err != nil {
		t.Fatal(err)
	}
	if !reliableNuma() {
		t.Skip("numa node counts are not reliable on this VM/OS combination")
	}
	if numa != s.numa {
		t.Errorf("got %d numa nodes, want %d", numa, s.numa)
	}
}

func TestC3FamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["C3"])
}
func TestC3FamilyMem(t *testing.T) {
	testMem(t, x86shapes["C3"])
}
func TestC3FamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["C3"])
}

func TestC3DFamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["C3D"])
}
func TestC3DFamilyMem(t *testing.T) {
	testMem(t, x86shapes["C3D"])
}
func TestC3DFamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["C3D"])
}

func TestE2FamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["E2"])
}
func TestE2FamilyMem(t *testing.T) {
	testMem(t, x86shapes["E2"])
}
func TestE2FamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["E2"])
}

func TestN2FamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["N2"])
}
func TestN2FamilyMem(t *testing.T) {
	testMem(t, x86shapes["N2"])
}
func TestN2FamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["N2"])
}

func TestN2DFamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["N2D"])
}
func TestN2DFamilyMem(t *testing.T) {
	testMem(t, x86shapes["N2D"])
}
func TestN2DFamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["N2D"])
}

func TestT2AFamilyCpu(t *testing.T) {
	testCpu(t, armshapes["T2A"])
}
func TestT2AFamilyMem(t *testing.T) {
	testMem(t, armshapes["T2A"])
}
func TestT2AFamilyNuma(t *testing.T) {
	testNuma(t, armshapes["T2A"])
}

func TestT2DFamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["T2D"])
}
func TestT2DFamilyMem(t *testing.T) {
	testMem(t, x86shapes["T2D"])
}
func TestT2DFamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["T2D"])
}

func TestN1FamilyCpu(t *testing.T) {
	testCpu(t, x86shapes["N1"])
}
func TestN1FamilyMem(t *testing.T) {
	testMem(t, x86shapes["N1"])
}
func TestN1FamilyNuma(t *testing.T) {
	testNuma(t, x86shapes["N1"])
}
