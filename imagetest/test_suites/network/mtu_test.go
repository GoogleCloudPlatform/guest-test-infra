//go:build cit
// +build cit

package network

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	gceMTU = 1460
)

func TestDefaultMTU(t *testing.T) {
	iface, err := utils.GetInterface(utils.Context(t), 0)
	if err != nil {
		t.Fatalf("couldn't find primary NIC: %v", err)
	}
	if utils.IsWindows() {
		sysprepInstalled, err := utils.RunPowershellCmd(`googet installed google-compute-engine-sysprep.noarch | Select-Object -Index 1`)
		if err != nil {
			t.Fatalf("could not check installed sysprep version: %v", err)
		}
		// YYYYMMDD
		sysprepVerRe := regexp.MustCompile("[0-9]{8}")
		sysprepVer, err := strconv.Atoi(sysprepVerRe.FindString(sysprepInstalled.Stdout))
		if err != nil {
			t.Fatalf("could not determine value of sysprep version: %v", err)
		}
		if sysprepVer <= 20240104 {
			t.Skipf("version %d of gcesysprep is too old to set interface mtu correctly", sysprepVer)
		}
	}
	if iface.MTU != gceMTU {
		t.Fatalf("expected MTU %d on interface %s, got MTU %d", gceMTU, iface.Name, iface.MTU)
	}
}
