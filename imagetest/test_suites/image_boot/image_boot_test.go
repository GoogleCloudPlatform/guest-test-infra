package imageboot

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

const (
	markerFile     = "/boot-marker"
	secureBootFile = "/sys/firmware/efi/vars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c/data"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

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

func TestGuestSecureBoot(t *testing.T) {
	if _, err := os.Stat(secureBootFile); os.IsNotExist(err) {
		t.Skip("not supported on non-uefi boot disk")
	}
	data, err := ioutil.ReadFile(secureBootFile)
	if err != nil {
		t.Fatal("failed reading secure boot file")
	}
	// https://www.kernel.org/doc/Documentation/ABI/stable/sysfs-firmware-efi-vars
	if data[0] != 1 {
		t.Fatal("secure boot is not enabled as expected")
	}
}
