package imageboot

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "imageboot"

var sbUnsupported = []*regexp.Regexp{
	// Permanent exceptions
	regexp.MustCompile("debian-1[01].*arm64"),
	regexp.MustCompile("windows-server-2012-r2-dc-core"), // Working but not easily testable and EOL in 1.5 months
	// Temporary exceptions
	// Waiting on MSFT signed shims:
	regexp.MustCompile("rocky-linux-[89].*arm64"),        // https://bugs.rockylinux.org/view.php?id=4027
	regexp.MustCompile("rhel-9.*arm64"),                  // https://issues.redhat.com/browse/RHEL-4326
	regexp.MustCompile("(sles-15|opensuse-leap).*arm64"), // https://bugzilla.suse.com/show_bug.cgi?id=1214761
}

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	vm, err := t.CreateTestVM("boot")
	if err != nil {
		return err
	}
	if err := vm.Reboot(); err != nil {
		return err
	}
	vm.RunTests("TestGuestBoot|TestGuestReboot$")

	vm2, err := t.CreateTestVM("guestreboot")
	if err != nil {
		return err
	}
	vm2.RunTests("TestGuestRebootOnHost")

	vm3, err := t.CreateTestVM("boottime")
	if err != nil {
		return err
	}
	vm3.AddMetadata("start-time", strconv.Itoa(time.Now().Second()))
	vm3.RunTests("TestStartTime|TestBootTime")

	if !strings.Contains(t.Image.Name, "sles") && !strings.Contains(t.Image.Name, "rhel-8-2-sap") && !strings.Contains(t.Image.Name, "rhel-8-1-sap") && !strings.Contains(t.Image.Name, "debian-10") && !strings.Contains(t.Image.Name, "ubuntu-pro-1804-lts-arm64") {
		suspend := &daisy.Instance{}
		suspend.Scopes = append(suspend.Scopes, "https://www.googleapis.com/auth/cloud-platform")
		suspendvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "suspend"}}, suspend)
		if err != nil {
			return err
		}
		suspendvm.RunTests("TestSuspend")
		suspendvm.Resume()
	}

	lm := &daisy.Instance{}
	lm.Scopes = append(lm.Scopes, "https://www.googleapis.com/auth/cloud-platform")
	lm.Scheduling = &compute.Scheduling{OnHostMaintenance: "MIGRATE"}
	lmvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "livemigrate"}}, lm)
	if err != nil {
		return err
	}
	lmvm.RunTests("TestLiveMigrate")

	for _, r := range sbUnsupported {
		if r.MatchString(t.Image.Name) {
			return nil
		}
	}
	if !utils.HasFeature(t.Image, "UEFI_COMPATIBLE") {
		return nil
	}
	vm4, err := t.CreateTestVM("secureboot")
	if err != nil {
		return err
	}
	vm4.EnableSecureBoot()
	vm4.RunTests("TestGuestSecureBoot")
	return nil
}
