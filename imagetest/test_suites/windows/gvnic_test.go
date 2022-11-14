//go:build cit
// +build cit

package windows

import (
	"errors"
	"fmt"
	"testing"
	"strings"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func CheckGVNICPresent(interfaceName string, t *testing.T) error {
	command := fmt.Sprintf("Get-NetAdapter -Name \"%s\"", interfaceName)
	result, err := utils.RunPowershellCmd(command)
	if err != nil {
		return err
	}
	if strings.Contains(result.Stdout,"Google Ethernet Adapter") {
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
	if err := CheckGVNICPresent(iface.Name, t); err!=nil {
		t.Fatalf("Error : %v", err.Error())
	}
	if err:= PingTest(); err != nil {
		t.Fatalf("ping test error : %v", err.Error())
	}
}

func PingTest() error {
	command := fmt.Sprintf("ping -S %s -w 2999 -n 5 %s",ip1,ip2)
	_, err := utils.RunPowershellCmd(command)
	return err
}

// Dummy test for target VM.
func TestEmptyTest(t *testing.T) {
	t.Log("vm boot successfully")
}
