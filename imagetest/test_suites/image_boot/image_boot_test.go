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
	t.Log("this test signal the guest boot successful")
}

func TestGuestReboot(t *testing.T) {
	_, err := os.Stat("/boot")
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create("/boot"); err != nil {
			t.Fatal("fail to create file when first boot")
		}
		t.Fatal("fail since the file does not exist")
	} else {
		// second boot
		t.Log("the file exist signal the guest reboot successful")
	}
}
