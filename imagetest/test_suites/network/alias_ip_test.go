package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	markerFile = "/boot-marker"
)

func TestAliasAfterReboot(t *testing.T) {
	var networkInterface string
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "debian-10"), strings.Contains(image, "ubuntu"):
		networkInterface = defaultPredictableInterface
	default:
		networkInterface = defaultInterface
	}

	_, err = os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatalf("failed on first boot")
	}
	// second boot
	actual, err := getGoogleRoutes(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := utils.GetMetadata("network-interfaces/0/ip-aliases/0")
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("alias ip is not as expected after reboot, expected %s, actual %s", expected, actual)
	}
}

func getGoogleRoutes(networkInterface string) (string, error) {
	arguments := strings.Split(fmt.Sprintf("route list table local type local scope host dev %s proto 66", networkInterface), " ")
	cmd := exec.Command("ip", arguments...)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", fmt.Errorf("alias IPs not configured")
	}
	return strings.Split(string(b), " ")[1], nil
}

func TestAliasAgentRestart(t *testing.T) {
	var networkInterface string

	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "debian-10") || strings.Contains(image, "ubuntu"):
		networkInterface = defaultPredictableInterface
	default:
		networkInterface = defaultInterface
	}

	beforeRestart, err := getGoogleRoutes(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("systemctl", "restart", "google-guest-agent")
	_, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	afterRestart, err := getGoogleRoutes(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	if beforeRestart != afterRestart {
		t.Fatalf("routes are inconsistent after restart, before %s after %s", beforeRestart, afterRestart)
	}
}
