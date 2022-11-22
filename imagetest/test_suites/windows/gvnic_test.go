//go:build cit
// +build cit

package windows

import (
	"errors"
	"fmt"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"strings"
	"testing"
)

func CheckGVNICPresent(interfaceName string, t *testing.T) error {
	command := fmt.Sprintf("Get-NetAdapter -Name \"%s\"", interfaceName)
	result, err := utils.RunPowershellCmd(command)
	if err != nil {
		return err
	}
	if strings.Contains(result.Stdout, "Google Ethernet Adapter") {
		return nil
	}
	return errors.New("GVNIC not present")
}

func TestGVNIC(t *testing.T) {
	utils.WindowsOnly(t)
	iface, err := utils.GetInterface(0)
	if err != nil {
		t.Fatalf("couldn't find primary NIC: %v", err)
	}
	if err := CheckGVNICPresent(iface.Name, t); err != nil {
		t.Fatalf("Error : %v", err.Error())
	}
}
