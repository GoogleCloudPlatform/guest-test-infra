//go:build cit
// +build cit

package packagevalidation

import (
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	chronyService    = "chronyd"
	ntpService       = "ntp"
	ntpdService      = "ntpd"
	chronycCmd       = "chronyc"
	ntpqCmd          = "ntpq"
	systemdTimesyncd = "systemd-timesyncd"
	timedatectlCmd   = "timedatectl"
)

func TestNTP(t *testing.T) {
	if runtime.GOOS == "windows" {
		testNTPWindows(t)
	} else {
		testNTPServiceLinux(t)
	}
}

// testNTPService Verify that ntp package exist and configuration is correct.
// debian 9, ubuntu 16.04 ntp
// debian 12 systemd-timesyncd
// sles-12 ntpd
// other distros chronyd
func testNTPServiceLinux(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata")
	}
	var servicename string
	switch {
	case strings.Contains(image, "debian-12"):
		servicename = systemdTimesyncd
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
	} else if utils.CheckLinuxCmdExists(timedatectlCmd) {
		cmd = exec.Command(timedatectlCmd, "show-timesync", "--property=FallbackNTPServers")
	} else {
		t.Fatalf("failed to find timedatectl chronyc or ntpq cmd")
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

func testNTPWindows(t *testing.T) {
	command := "w32tm /query /peers /verbose"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting NTP information: %v", err)
	}

	expected := []string{
		"#Peers: 1",
		"Peer: metadata.google.internal,0x1",
		"LastSyncErrorMsgId: 0x00000000 (Succeeded)",
	}

	for _, exp := range expected {
		if !strings.Contains(output.Stdout, exp) {
			t.Fatalf("Expected info %s not found in peer_info: %s", exp, output.Stdout)
		}
	}

	// NTP can take time to get to an active state.
	if !(strings.Contains(output.Stdout, "State: Active") || strings.Contains(output.Stdout, "State: Pending")) {
		t.Fatalf("Expected State: Active or Pending in: %s", output.Stdout)
	}

	r, err := regexp.Compile("Time Remaining: ([0-9\\.]+)s")
	if err != nil {
		t.Fatalf("Error creating regexp: %v", err)
	}

	remaining := r.FindStringSubmatch(output.Stdout)[1]
	remainingTime, err := strconv.ParseFloat(remaining, 32)
	if err != nil {
		t.Fatalf("Unexpected remaining time value: %s", remaining)
	}

	if remainingTime < 0.0 {
		t.Fatalf("Invalid remaining time: %f", remainingTime)
	}

	if remainingTime > 900.0 {
		t.Fatalf("Time remaining is longer than the 15 minute poll interval: %f", remainingTime)
	}
}
