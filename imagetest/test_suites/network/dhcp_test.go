// +build cit
// +build linux_test

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
		if strings.Contains(line, fmt.Sprintf("dhclient %s", iface.Name)) {
			return
		}
	}
	t.Fatalf("failed finding dhclient process")
}
