package metadata

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	shutdownScriptTime = `#!/bin/bash

while [[ 1 ]]; do
  date +%s >> /shutdown.txt
  sync
  sleep 1
done`
	shutdownScriptTemplate = `#!/bin/bash
echo "%s" > %s`
	shutdownOutputPath = "/shutdown_out.txt"
	shutdownContent    = "The shutdown script worked."
	shutdownMaxLength  = 32768 // max shutdown metadata value
)

var shutdownScript = fmt.Sprintf(shutdownScriptTemplate, shutdownContent, shutdownOutputPath)

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
	vm2.SetShutdownScript(shutdownScript)
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestShutdownScript$")

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.SetShutdownScript(strings.Repeat("a", shutdownMaxLength))
	if err := vm3.Reboot(); err != nil {
		return err
	}
	vm3.RunTests("TestShutdownScriptFailed")

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	if err := vm4.SetShutdownScriptURL(shutdownScript); err != nil {
		return err
	}
	if err := vm4.Reboot(); err != nil {
		return err
	}
	vm4.RunTests("TestShutdownUrlScript")

	vm5, err := t.CreateTestVM("vm5")
	if err != nil {
		return err
	}
	vm5.SetShutdownScript(shutdownScriptTime)
	if err := vm5.Reboot(); err != nil {
		return err
	}
	vm5.RunTests("TestShutdownScriptTime")
	return nil
}
