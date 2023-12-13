//go:build cit
// +build cit

package disk

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestBlockDeviceNaming(t *testing.T) {
	utils.LinuxOnly(t)
	err := exec.Command("udevadm", "trigger").Run()
	if err != nil {
		t.Fatal(err)
	}
	err = exec.Command("udevadm", "settle").Run()
	if err != nil {
		t.Fatal(err)
	}
	disks, err := os.ReadDir("/dev/disk/by-id")
	if err != nil {
		t.Fatal(err)
	}
	var disklist []string
	for _, disk := range disks {
		disklist = append(disklist, disk.Name())
		if disk.Name() == "google-secondary" {
			return
		}
	}
	t.Fatalf("could not find a disk named google-secondary, found these disks: %s", strings.Join(disklist, " "))
}
