//go:build cit
// +build cit

package network

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	driverInfoPath = "/sys/class/net/%s/device/uevent"
)

var (
	driverRegex = regexp.MustCompile("DRIVER=[a-zA-Z0-9_-]+")
)

func CheckGVNICPresent(interfaceName string) error {
	file := fmt.Sprintf(driverInfoPath, interfaceName)
	_, err := os.Stat(file)
	if os.IsNotExist(err) {
		return err
	}
	fileData, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	driver := driverRegex.FindString(string(fileData))
	if driver == "" {
		return errors.New("No network driver information found")
	}
	if !strings.Contains(driver, "gvnic") {
		errMsg := fmt.Sprintf("Driver set as %v want gvnic", driver[7:])
		return errors.New(errMsg)
	}
	return nil
}

func TestGVNIC(t *testing.T) {
	iface, err := utils.GetInterface(0)
	if err != nil {
		t.Fatalf("couldn't find primary NIC: %v", err)
	}
	if err := CheckGVNICPresent(iface.Name); err != nil {
		t.Fatalf("Error : %v", err.Error())
	}
}
