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
	if !utils.HasFeature(t.Image, "SEV_CAPABLE") {
		t.Skip(fmt.Sprintf("%s does not support CVM", t.Image.Name))
	}
	vm, err := t.CreateTestVM(vmName)
	if err != nil {
		return err
	}
	vm.EnableConfidentialInstance()
	vm.SetMinCPUPlatform("AMD Milan")
	vm.ForceMachineType("n2d-standard-2")
	vm.RunTests("TestCVMEnabled")
	return nil
}
