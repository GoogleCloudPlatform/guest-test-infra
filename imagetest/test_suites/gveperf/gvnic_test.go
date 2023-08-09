//go:build cit
// +build cit

package gveperf

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	driverPath = "/sys/class/net/%s/device/driver"
)

func CheckGVNICPresent(interfaceName string) error {
	file := fmt.Sprintf(driverPath, interfaceName)
	data, err := os.Readlink(file)
	if err != nil {
		return err
	}
	s := strings.Split(data, "/")
	driver := s[len(s)-1]
	if driver != "gvnic" {
		errMsg := fmt.Sprintf("Driver set as %v want gvnic", driver)
		return errors.New(errMsg)
	}
	return nil
}

func CheckGVNICPresentWindows(interfaceName string) error {
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

func CheckGVNICPerformance() (string, error) {
	// By the time the test runs, the startupscripts have already finished.
	results, err := utils.GetMetadataGuestAttribute("testing/results")
	if err != nil {
		return "", err
	}
	return results, nil
}

func TestGVNICExists(t *testing.T) {
	iface, err := utils.GetInterface(0)
	if err != nil {
		t.Fatalf("couldn't find primary NIC: %v", err)
	}
	var errMsg error
	if runtime.GOOS == "windows" {
		errMsg = CheckGVNICPresentWindows(iface.Name)
	} else {
		errMsg = CheckGVNICPresent(iface.Name)
	}
	if errMsg != nil {
		t.Fatalf("Error: %v", errMsg.Error())
	}
}

func TestGVNICPerformance(t *testing.T) {
	// Check performance of the driver.
	if runtime.GOOS != "windows" {
		results, err := CheckGVNICPerformance()
		if err != nil {
			t.Fatalf("Error : %v", err)
		}
		t.Logf(results)
	}
}
