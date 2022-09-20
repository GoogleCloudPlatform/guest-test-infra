//go:build cit
// +build cit

package windows

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func getDriverList(remove bool) []string {
	drivers := []string{
		"google-compute-engine-driver-netkvm",
		"google-compute-engine-driver-pvpanic",
		"google-compute-engine-driver-gga",
		"google-compute-engine-driver-balloon",
	}

	// Do not remove Network or Disk
	if !remove {
		drivers = append(
			drivers,
			"google-compute-engine-driver-gvnic",
			"google-compute-engine-driver-vioscsi",
		)
	}
	return drivers
}

func getInstalledDrivers() ([]string, error) {
	command := fmt.Sprintf("%s installed | Select-String google-compute-engine-driver-", googet)
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		return nil, err
	}

	drivers := strings.Split(output.Stdout, "\n")

	return drivers, nil

}

func TestNetworkDriverLoaded(t *testing.T) {
	utils.WindowsOnly(t)
	command := fmt.Sprintf("ipconfig /all | Select-String Description")
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error running 'ipconfig': %v", err)
	}
	adapterName := "Google VirtIO Ethernet Adapter"
	if !strings.Contains(output.Stdout, adapterName) {
		t.Fatalf("Stdout: %s does not contain '%s'", output.Stdout, adapterName)
	}
}

func TestDriversInstalled(t *testing.T) {
	utils.WindowsOnly(t)
	driverList := getDriverList(false)
	installedDriverList, err := getInstalledDrivers()
	if err != nil {
		t.Fatalf("Cannot get installed drivers list: %v", err)
	}
	for _, driver := range driverList {
		driverInstalled := false
		for _, installed := range installedDriverList {
			if strings.Contains(installed, driver) {
				driverInstalled = true
				break
			}
		}
		if !driverInstalled {
			t.Fatalf("Driver '%s' is not installed", driver)
		}
	}
}

func TestDriversRemoved(t *testing.T) {
	utils.WindowsOnly(t)
	driverList := getDriverList(true)
	for _, driver := range driverList {
		command := fmt.Sprintf("%s -noconfirm remove %s", googet, driver)
		output, err := utils.RunPowershellCmd(command)
		if err != nil {
			t.Fatalf("Error removing '%s': %v", driver, err)
		}
		rmString := fmt.Sprintf("Removal of %s completed", driver)
		if !strings.Contains(output.Stdout, rmString) {
			t.Fatalf("Cannot confirm removal of '%s': %s", driver, output.Stdout)
		}
	}
}
