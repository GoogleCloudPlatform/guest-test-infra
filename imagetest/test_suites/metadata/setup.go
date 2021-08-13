package metadata

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	startupScriptTemplate = `#!/bin/bash
echo "%s" > %s`
	startupOutputPath = "/startup_out.txt"
	startupContent    = "The startup script worked."
	startupMaxLength  = 32768 // max shutdown metadata value
)

var startupScript = fmt.Sprintf(startupScriptTemplate, startupContent, startupOutputPath)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.RunTests("TestTokenFetch|TestMetaDataResponseHeaders|TestGetMetaDataUsingIP")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.SetStartupScript(startupScript)
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestStartupScript$")

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.SetShutdownScript(strings.Repeat("a", startupMaxLength))
	if err := vm3.Reboot(); err != nil {
		return err
	}
	vm3.RunTests("TestStartupScriptFailed")

	return nil
}
