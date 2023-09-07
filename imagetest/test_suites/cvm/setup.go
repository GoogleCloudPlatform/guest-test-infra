package setup

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

var Name = "cvm"

const vmName = "vm"

// TestSetup sets up test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// TODO -- Workflow passes full image URL, so use that to find if it's
	// in the list of valid images
	if strings.Contains(t.Image, "windows") {
		t.Skip(fmt.Sprintf("%v does not support CVM", t.Image))
	}

	vm := t.CreateTestVM(vmName)
	if err != nil {
		return err
	}
	vm.EnableConfidentialInstance()
	vm.SetMinCpuPlatform("AMD Milan")
	vm.SetMachineType("n2d-standard-16")

	vm.RunTests("TestCVMEnabled")
	return nil
}
