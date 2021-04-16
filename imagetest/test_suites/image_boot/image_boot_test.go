// +build !linux

package imageboot

import (
	"os"
	"syscall"
	"testing"
)

func TestGuestBoot(t *testing.T) {
	err := syscall.Uname(&syscall.Utsname{})

	if err != nil {
		t.Fatalf("couldn't get system information, image boot failed")
	}

}

func TestGuestReboot(t *testing.T) {
	_, err := os.Stat("/tmp/boot")
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create("/tmp/boot"); err != nil {
			t.Fatal("fail to create file when first boot")
			return
		}
		return
	}
	// second boot
	err = syscall.Uname(&syscall.Utsname{})

	if err != nil {
		t.Fatalf("couldn't get system information, image reboot failed")
	}
}
