package cvm

import (
	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	computeBeta "google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "cvm"

const vmName = "vm"

// TestSetup sets up test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if t.Image.Architecture == "ARM64" {
		t.Skip("CVM is not supported on arm")
	}
	for _, feature := range t.Image.GuestOsFeatures {
		switch feature.Type {
		case "SEV_CAPABLE":
			sevtests := "TestSEVEnabled"
			vm := &daisy.InstanceBeta{}
			vm.Name = vmName + "-SEV"
			vm.ConfidentialInstanceConfig = &computeBeta.ConfidentialInstanceConfig{
				ConfidentialInstanceType:  "SEV",
				EnableConfidentialCompute: true,
			}
			vm.Scheduling = &computeBeta.Scheduling{OnHostMaintenance: "TERMINATE"}
			vm.MachineType = "n2d-standard-2"
			vm.MinCpuPlatform = "AMD Milan"
			disks := []*compute.Disk{&compute.Disk{Name: vmName + "-SEV", Type: imagetest.PdBalanced}}
			tvm, err := t.CreateTestVMFromInstanceBeta(vm, disks)
			if err != nil {
				return err
			}
			tvm.RunTests(sevtests)
		case "SEV_SNP_CAPABLE":
			vm := &daisy.InstanceBeta{}
			vm.Name = vmName + "-SEVSNP"
			vm.Zone = "us-central1-a" // SEV_SNP not available in all regions
			vm.ConfidentialInstanceConfig = &computeBeta.ConfidentialInstanceConfig{
				ConfidentialInstanceType:  "SEV_SNP",
				EnableConfidentialCompute: true,
			}
			vm.Scheduling = &computeBeta.Scheduling{OnHostMaintenance: "TERMINATE"}
			vm.MachineType = "n2d-standard-2"
			vm.MinCpuPlatform = "AMD Milan"
			disks := []*compute.Disk{
				&compute.Disk{Name: vmName + "-SEVSNP", Type: imagetest.PdBalanced, Zone: "us-central1-a"},
			}
			tvm, err := t.CreateTestVMFromInstanceBeta(vm, disks)
			if err != nil {
				return err
			}
			tvm.RunTests("TestSEVSNPEnabled")
		case "TDX_CAPABLE":
			vm := &daisy.InstanceBeta{}
			vm.Name = vmName + "-TDX"
			vm.Zone = "us-central1-a" // TDX not available in all regions
			vm.ConfidentialInstanceConfig = &computeBeta.ConfidentialInstanceConfig{
				ConfidentialInstanceType:  "TDX",
				EnableConfidentialCompute: true,
			}
			vm.Scheduling = &computeBeta.Scheduling{OnHostMaintenance: "TERMINATE"}
			vm.MachineType = "c3-standard-2"
			vm.MinCpuPlatform = "Intel Sapphire Rapids"
			disks := []*compute.Disk{
				&compute.Disk{Name: vmName + "-TDX", Type: imagetest.PdBalanced, Zone: "us-central1-a"},
			}
			tvm, err := t.CreateTestVMFromInstanceBeta(vm, disks)
			if err != nil {
				return err
			}
			tvm.RunTests("TestTDXEnabled")
		}
	}
	return nil
}
