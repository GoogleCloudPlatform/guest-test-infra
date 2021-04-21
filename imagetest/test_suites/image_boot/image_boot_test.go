package imageboot

import (
	"bytes"
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

const markerFile = "/boot-marker"

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

func TestGuestSecureReboot(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	if !isSecureBootEnabled() {
		t.Fatal("secure boot is not enabled as expected")
	}
	t.Log("verify secure boot enabled after reboot")
}

func isSecureBootEnabled() bool {
	time.Sleep(30)
	cmd := exec.Command("hexdmp", "/sys/firmware/efi/vars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c/data")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	return strings.Contains(out.String(), "0000000 0001")
}
