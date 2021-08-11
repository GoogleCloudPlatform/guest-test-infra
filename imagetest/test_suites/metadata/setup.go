package metadata

import (
	"fmt"
	"math/rand"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	shutdownScript = `#!/bin/bash

while [[ 1 ]]; do
  date +%s >> /shutdown.txt
  sync
  sleep 1
done`
	shutdownScriptTemplate = `#!/bin/bash
echo "%s" > %s`
	shutdownContent    = "The shutdown script worked."
	shutdownOutputPath = "/shutdown_out.txt"
	shutdownMaxLength  = 32768 // max shutdown metadata value
	shutdownScriptURL  = "gs://gcp-guest-cloud-test-artifacts/shutdown-script.txt"
)

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
	vm2.SetShutdownScript(fmt.Sprintf(shutdownScriptTemplate, shutdownContent, shutdownOutputPath))
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestShutdownScript")

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.SetShutdownScript(randStringRunes(shutdownMaxLength))
	if err := vm3.Reboot(); err != nil {
		return err
	}
	vm3.RunTests("TestRandomShutdownScriptNotCrashVM")

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	vm4.SetShutdownScriptURL(shutdownScriptURL)
	if err := vm4.Reboot(); err != nil {
		return err
	}
	vm4.RunTests("TestShutdownUrlScript")

	vm5, err := t.CreateTestVM("vm5")
	if err != nil {
		return err
	}
	vm5.SetShutdownScript(shutdownScript)
	if err := vm5.Reboot(); err != nil {
		return err
	}
	vm5.RunTests("TestGuestShutdownScript")
	return nil
}

func randStringRunes(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
