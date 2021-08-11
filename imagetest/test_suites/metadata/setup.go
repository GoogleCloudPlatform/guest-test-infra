package metadata

import (
	"fmt"
	"math/rand"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "metadata"

const (
	shutdownScriptTemplate = `#!/bin/bash
echo "%s" > %s
`
	shutdownContent = "The shutdown script worked."
	outputPath      = "/shutdown_out.txt"
	// Max metadata value
	maxLength = 32768
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	_, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.SetShutdownScript(fmt.Sprintf(shutdownScriptTemplate, shutdownContent, outputPath))
	if err := vm2.Reboot(); err != nil {
		return err
	}
	vm2.RunTests("TestShutdownScript")

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.SetShutdownScript(randStringRunes(maxLength))
	if err := vm3.Reboot(); err != nil {
		return err
	}
	vm3.RunTests("TestRandomShutdownScriptNotCrashVM")

	vm4, err := t.CreateTestVM("vm4")
	if err != nil {
		return err
	}
	vm4.SetShutdownScriptURL("url")
	if err := vm4.Reboot(); err != nil {
		return err
	}
	vm4.RunTests("TestShutdownUrlScript")
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
