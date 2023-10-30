package cvm

import (
	"fmt"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "cvm"

const vmName = "vm"

// TestSetup sets up test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if t.Image.Architecture == "ARM64" {
		t.Skip("CVM is not supported on arm")
	}
	var cvm_support bool
	for _, feature := range t.Image.Features {
		switch feature {
		case "SEV_CAPABLE":
			cvm_support = true
			// TODO test live migration
			vm, err := t.CreateTestVM(vmName+"-SEV")
			if err != nil {
				return err
			}
			vm.EnableConfidentialInstance()
			vm.SetMinCPUPlatform("AMD Milan")
			vm.ForceMachineType("n2d-standard-2")
			vm.RunTests("TestSEVEnabled")
		case "SEV_SNP_CAPABLE":
			cvm_support = true
			vm, err := t.CreateTestVM(vmName+"-SEVSNP")
			if err != nil {
				return err
			}
			vm.EnableConfidentialInstance() // DO NOT SUBMIT
			// This doesn't work here, need to set confidentialInstanceConfig.ConfidentialInstanceType = SEV_SNP
			vm.SetMinCPUPlatform("AMD Milan")
			vm.ForceMachineType("n2d-standard-2")
			vm.RunTests("TestSEVEnabled")
		case "TDX_CAPABLE":
			cvm_support = true
			vm, err := t.CreateTestVM(vmName+"-TDX")
			if err != nil {
				return err
			}
			vm.EnableConfidentialInstance() // DO NOT SUBMIT
			// This doesn't work here, need to set confidentialInstanceConfig.ConfidentialInstanceType = TDX
			vm.SetMinCPUPlatform("Intel Sapphire Rapids")
			vm.ForceMachineType("c3-standard-2")
			vm.RunTests("TestTDXEnabled")
		}
	}
	if !cvm_support {
		t.Skip(fmt.Sprintf("%s does not support CVM", t.Image.Name))
	}
	return nil
}
