package cvm

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "cvm"

const vmName = "vm"

// TestSetup sets up test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if strings.Contains(t.Image, "windows") || strings.Contains(t.Image, "rhel-7") || strings.Contains(t.Image, "centos-7") || strings.Contains(t.Image, "debian-10") {
		t.Skip(fmt.Sprintf("%v does not support CVM", t.Image))
	}

	vm, err := t.CreateTestVM(vmName)
	if err != nil {
		return err
	}
	vm.EnableConfidentialInstance()
	vm.SetMinCPUPlatform("AMD Milan")
	vm.SetMachineType("n2d-standard-16")

	vm.RunTests("TestCVMEnabled")
	return nil
}
