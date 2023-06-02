//go:build cit
// +build cit

package network

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	dhclient = "dhclient"
	wicked   = "wicked"
)

// TestDHCP test secondary interfaces are configured with a single dhclient process.
func TestDHCP(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	// Which dhcp client the guest agent uses for multinic bringup.
	var dhcpclient string
	switch {
	case strings.Contains(image, "sles"):
		dhcpclient = wicked
	default:
		dhcpclient = dhclient
	}

	iface, err := utils.GetInterface(1)
	if err != nil {
		t.Fatalf("couldn't get secondary interface: %v", err)
	}

	cmd := exec.Command("ps", "x")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ps command failed %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, fmt.Sprintf("%s %s", dhcpclient, iface.Name)) {
			return
		}
	}
	t.Fatalf("failed finding dhcp client process")
}
