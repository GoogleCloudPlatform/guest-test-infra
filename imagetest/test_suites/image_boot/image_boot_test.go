// +build cit

package imageboot

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	markerFile     = "/boot-marker"
	secureBootFile = "/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"
)

func TestGuestBoot(t *testing.T) {
	t.Log("Guest booted successfully")
}

func TestGuestReboot(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	t.Log("marker file exist signal the guest reboot successful")
}

func TestGuestRebootOnHost(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		cmd := exec.Command("sudo", "nohup", "reboot")
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run reboot command: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	t.Log("marker file exist signal the guest reboot successful")
}

func TestGuestSecureBoot(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	if strings.Contains(image, "debian-9") {
		t.Skip("secure boot is not supported on Debian 9")
	}

	if _, err := os.Stat(secureBootFile); os.IsNotExist(err) {
		t.Fatal("efi var is missing")
	}
	data, err := ioutil.ReadFile(secureBootFile)
	if err != nil {
		t.Fatal("failed reading secure boot file")
	}
	// https://www.kernel.org/doc/Documentation/ABI/stable/sysfs-firmware-efi-vars
	if data[len(data)-1] != 1 {
		t.Fatal("secure boot is not enabled as expected")
	}
}

func TestBootTime(t *testing.T) {
	metadata, err := utils.GetMetadataAttribute("start-time")
	if err != nil {
		t.Fatalf("couldn't get start time from metadata")
	}
	startTime, err := strconv.Atoi(metadata)
	if err != nil {
		t.Fatalf("failed to convet start time %s", metadata)
	}
	t.Logf("image boot time is %d", time.Now().Second()-startTime)
}
