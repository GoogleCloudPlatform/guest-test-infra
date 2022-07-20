// +build cit

package network

import (
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	gceMTU = 1500
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
