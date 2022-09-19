package imageboot

import (
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "image_boot"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	if err := vm.Reboot(); err != nil {
		return err
	}
	vm.RunTests("TestGuestBoot|TestGuestReboot$")

	vm2, err := t.CreateTestVM("vm2")
	if err != nil {
		return err
	}
	vm2.RunTests("TestGuestRebootOnHost")

	if strings.Contains(t.Image, "debian-9") {
		// secure boot is not supported on Debian 9
		return nil
	}

	if strings.Contains(t.Image, "rocky-linux-8") {
		// secure boot is not supported on Rocky Linux
		return nil
	}

	if strings.Contains(t.Image, "almalinux") {
		// secure boot is not supported on Alma Linux
		return nil
	}

	if strings.Contains(t.Image, "arm64") {
		// secure boot is not supported on ARM images
		return nil
	}

	vm3, err := t.CreateTestVM("vm3")
	if err != nil {
		return err
	}
	vm3.AddMetadata("start-time", strconv.Itoa(time.Now().Second()))
	vm3.EnableSecureBoot()
	vm3.RunTests("TestGuestSecureBoot|TestStartTime|TestBootTime")
	return nil
}
