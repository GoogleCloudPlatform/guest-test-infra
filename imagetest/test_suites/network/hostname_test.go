package network

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestHostname(t *testing.T) {
	metadataHostname, err := utils.GetMetadata("hostname")
	if err != nil {
		t.Fatalf(" still couldn't determine metadata hostname")
	}

	// 'hostname' in metadata is fully qualified domain name.
	shortname := strings.Split(metadataHostname, ".")[0]

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("couldn't determine local hostname")
	}

	if hostname != shortname {
		t.Errorf("hostname does not match metadata. Expected: %q got: %q", shortname, hostname)
	}

	// If hostname is FQDN then lots of tools (e.g. ssh-keygen) have issues
	if strings.Contains(hostname, ".") {
		t.Errorf("hostname contains '.'")
	}
}

// TestCustomHostname tests the 'fully qualified domain name', using the logic in the `hostname` utility.
func TestCustomHostname(t *testing.T) {
	TestFQDN(t)
}

// TestFQDN tests the 'fully qualified domain name', using the logic in the `hostname` utility.
func TestFQDN(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	if strings.Contains(image, "rhel-7-4-sap") {
		t.Skip("hostname is not working well on RHEL 7-4-Sap")
	}

	metadataHostname, err := utils.GetMetadata("hostname")
	if err != nil {
		t.Fatalf("couldn't determine metadata hostname")
	}

	// This command is not safe on multi-NIC VMs. See HOSTNAME(1), section 'THE FQDN'.
	cmd := exec.Command("/bin/hostname", "-A")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hostname command failed")
	}
	hostname := strings.TrimRight(string(out), " \n")

	if hostname != metadataHostname {
		t.Errorf("hostname does not match metadata. Expected: %q got: %q", metadataHostname, hostname)
	}
}
