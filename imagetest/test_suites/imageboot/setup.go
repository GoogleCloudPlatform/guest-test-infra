package imageboot

import (
	"regexp"
	"strconv"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "imageboot"

var sbUnsupported = []*regexp.Regexp{
	// Permanent exceptions
	regexp.MustCompile("debian-9"),
	regexp.MustCompile("debian-1[01].*arm64"),
	regexp.MustCompile("windows-server-2012-r2-dc-core"),
	// Temporary exceptions
	regexp.MustCompile("debian-12.*arm64"),
	// Waiting on MSFT signed shims:
	regexp.MustCompile("rocky-linux-[89].*arm64"), // https://bugs.rockylinux.org/view.php?id=4027
	regexp.MustCompile("rhel-9.*arm64"), // https://bugzilla.redhat.com/show_bug.cgi?id=2103803
	regexp.MustCompile("sles-15.*arm64"), // https://bugzilla.suse.com/show_bug.cgi?id=1214761
}

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

	for _, r := range sbUnsupported {
		if r.MatchString(t.Image) {
			return nil
		}
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
