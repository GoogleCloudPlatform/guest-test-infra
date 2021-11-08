// +build cit

package network

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestDHCP test secondary interfaces are configured with a single dhclient process.
func TestDHCP(t *testing.T) {
	var networkInterface string

	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "debian-10") || strings.Contains(image, "debian-11") || strings.Contains(image, "ubuntu"):
		networkInterface = "ens5"
	case strings.Contains(image, "sles"):
		t.Skip("dhclient test not supported on SLES")
	default:
		networkInterface = "eth1"
	}
	cmd := exec.Command("ps", "x")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ps command failed %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, fmt.Sprintf("dhclient %s", networkInterface)) {
			return
		}
	}
	t.Fatalf("failed finding dhclient process")
}
