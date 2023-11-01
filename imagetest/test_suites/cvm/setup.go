package cvm

import (
	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
	computeBeta "google.golang.org/api/compute/v0.beta"
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
			vm, err := t.CreateTestVM(vmName+"-SEV")
			if err != nil {
				return err
			}
			vm.EnableConfidentialInstance()
			vm.SetMinCPUPlatform("AMD Milan")
			vm.ForceMachineType("n2d-standard-2")
			vm.RunTests("TestSEVEnabled|TestLiveMigrate")
		case "SEV_SNP_CAPABLE":
			vm := &daisy.InstanceBeta{}
			vm.Name = vmName+"-SEVSNP"
			vm.Zone = "us-central1-a" // SEV_SNP not available in all regions
			vm.ConfidentialInstanceConfig = &computeBeta.ConfidentialInstanceConfig{
				ConfidentialInstanceType: "SEV_SNP",
				EnableConfidentialCompute: true,
			}
			vm.MachineType = "n2d-standard-2"
			vm.MinCpuPlatform = "AMD Milan"
			disks := []*compute.Disk{
				&compute.Disk{Name: vmName+"-SEVSNP", Type: imagetest.PdBalanced, Zone: "us-central1-a"},
			}
			tvm, err := t.CreateTestVMBeta(disks, vm)
			if err != nil {
				return err
			}
			tvm.RunTests("TestSEVSNPEnabled")*/
		case "TDX_CAPABLE":
			vm := &daisy.InstanceBeta{}
			vm.Name = vmName+"-TDX"
			vm.Zone = "us-central1-a" // TDX not available in all regions
			vm.ConfidentialInstanceConfig = &compute.ConfidentialInstanceConfig{
				ConfidentialInstanceType: "TDX",
				EnableConfidentialCompute: true,
			}
			vm.MachineType = "c3-standard-2"
			vm.MinCpuPlatform = "Intel Sapphire Rapids"
			disks := []*compute.Disk{
				&compute.Disk{Name: vmName+"-TDX", Type: imagetest.PdBalanced, Zone: "us-central1-a"},
			}
			tvm, err := t.CreateTestVMBeta(disks, vm)
			if err != nil {
				return err
			}
			tvm.RunTests("TestTDXEnabled")*/
		}
	}
	return nil
}
