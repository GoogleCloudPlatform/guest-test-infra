// +build cit
// +build linux_test

package network

import (
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	gceMTU = 1460
)

func TestDefaultMTU(t *testing.T) {
	iface, err := utils.GetInterface(0)
	if err != nil {
		t.Fatalf("couldn't find primary NIC: %v", err)
	}

	if iface.MTU != gceMTU {
		t.Fatalf("expected MTU %d on interface %s, got MTU %d", gceMTU, iface.Name, iface.MTU)
	}
}
