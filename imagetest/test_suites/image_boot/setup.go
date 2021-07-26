package imageboot

import (
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "image_boot"

const script = `#!/bin/bash

while [[ 1 ]]; do
  date +%s >> /shutdown.txt
  sync
  sleep 1
done`

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm.SetShutdownScript(script)
	if err := vm.Reboot(); err != nil {
		return err
	}
	vm.RunTests("TestGuestBoot|TestGuestReboot|TestGuestShutdownScript")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.RunTests("TestGuestRebootOnHost")

	if strings.Contains(t.Image, "debian-9") {
		t.Skip("secure boot is not supported on Debian 9")
	}

	if strings.Contains(t.Image, "rocky-linux-8") {
		t.Skip("secure boot is not supported on Rocky Linux")
	}

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.EnableSecureBoot()
	vm3.RunTests("TestGuestSecureBoot")
	return nil
}
