//go:build cit
// +build cit

package imagevalidation

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	chronyService = "chronyd"
	ntpService    = "ntp"
	ntpdService   = "ntpd"
	chronycCmd    = "chronyc"
	ntpqCmd       = "ntpq"
)

// TestNTPService Verify that ntp package exist and configuration is correct.
// debian 9, ubuntu 16.04 ntp
// sles-12 ntpd
// other distros chronyd
func TestNTPService(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata")
	}
	var servicename string
	switch {
	case strings.Contains(image, "debian-9"), strings.Contains(image, "ubuntu-pro-1604"):
		servicename = ntpService
	case strings.Contains(image, "sles-12"):
		servicename = ntpdService
	default:
		servicename = chronyService
	}
	var cmd *exec.Cmd
	if utils.CheckLinuxCmdExists(chronycCmd) {
		cmd = exec.Command(chronycCmd, "-c", "sources")
	} else if utils.CheckLinuxCmdExists(ntpqCmd) {
		cmd = exec.Command(ntpqCmd, "-np")
	} else {
		t.Fatalf("failed to find chronyc or ntpq cmd")
	}

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ntp command failed %v", err)
	}
	serverNames := []string{"metadata.google.internal", "metadata", "169.254.169.254"}
	foundNtpServer := false
	outputString := string(out)
	for _, serverName := range serverNames {
		if strings.Contains(outputString, serverName) {
			foundNtpServer = true
			break
		}
	}
	if !foundNtpServer {
		t.Fatalf("could not find ntp server")
	}

	// Make sure that ntp service is running.
	systemctlCmd := exec.Command("systemctl", "is-active", servicename)
	if err := systemctlCmd.Run(); err != nil {
		t.Fatalf("%s service is not running", servicename)
	}
}
