//go:build cit
// +build cit

package network

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	driverPath = "/sys/class/net/%s/device/driver"

	// Perf constants.
	maxRetries    = 10
	sleepDuration = 60 * time.Second
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
	// Wait for iperf to finish.
	for i := 0; i <= maxRetries; i++ {
		results, err := utils.GetMetadataGuestAttribute("testing/results")
		if err != nil {
			// As long as the test results do not exist, the test is not finished.
			if i == maxRetries-1 {
				return "", errors.New(fmt.Sprintf("Client VM not terminated after %v attempts", maxRetries))
			}
			time.Sleep(sleepDuration)
		} else {
			return fmt.Sprintf("Results: %s", results), nil
		}
	}
	return "", errors.New("Wait loop completed without returning. Failing.")
}

func TestGVNIC(t *testing.T) {
	iface, err := utils.GetInterface(0)

	// Check whether the driver exists.
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
		t.Fatalf("Error : %v", errMsg.Error())
	}

	// Check performance of the driver.
	results, err := CheckGVNICPerformance()
	if err != nil {
		t.Fatalf("Error : %v", err)
	}
	t.Logf(results)
}
