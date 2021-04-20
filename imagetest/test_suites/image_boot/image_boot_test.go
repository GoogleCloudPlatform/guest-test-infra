// +build linux

package imageboot

import (
	"flag"
	"os"
	"testing"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
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
	_, err := os.Stat("/boot-marker")
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create("/boot-marker"); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	} else {
		// second boot
		t.Log("marker file exist signal the guest reboot successful")
	}
}
