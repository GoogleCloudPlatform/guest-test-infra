// +build cit

package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	markerFile = "/boot-marker"
)

func TestAliases(t *testing.T) {
	if err := verifyIPAliases(); err != nil {
		t.Fatal(err)
	}
}

func TestAliasAfterReboot(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("first boot")
	}
	// second boot
	if err := verifyIPAliases(); err != nil {
		t.Fatal(err)
	}
}

func verifyIPAliases() error {
	var networkInterface string

	image, err := utils.GetMetadata("image")
	if err != nil {
		return fmt.Errorf("couldn't get image from metadata: %v", err)
	}
	switch {
	case strings.Contains(image, "debian-10"), strings.Contains(image, "debian-11"), strings.Contains(image, "ubuntu"):
		networkInterface = defaultPredictableInterface
	default:
		networkInterface = defaultInterface
	}

	actualIPs, err := getGoogleRoutes(networkInterface)
	if err != nil {
		return err
	}
	if err := verifyIPExist(actualIPs); err != nil {
		return err
	}
	return nil
}

func getGoogleRoutes(networkInterface string) ([]string, error) {
	// First, we probably need to wait so the guest agent can add the
	// routes. If this is insufficient, we might need to add retries.
	time.Sleep(30 * time.Second)

	arguments := strings.Split(fmt.Sprintf("route list table local type local scope host dev %s proto 66", networkInterface), " ")
	cmd := exec.Command("ip", arguments...)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("alias IPs not configured")
	}
	var res []string
	for _, line := range strings.Split(string(b), "\n") {
		ip := strings.Split(line, " ")
		if len(ip) >= 2 {
			res = append(res, ip[1])
		}
	}
	return res, nil
}

func TestAliasAgentRestart(t *testing.T) {
	var networkInterface string

	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "debian-10"), strings.Contains(image, "debian-11"), strings.Contains(image, "ubuntu"):
		networkInterface = defaultPredictableInterface
	default:
		networkInterface = defaultInterface
	}

	beforeRestart, err := getGoogleRoutes(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("systemctl", "restart", "google-guest-agent")
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	afterRestart, err := getGoogleRoutes(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	if !compare(beforeRestart, afterRestart) {
		t.Fatalf("routes are inconsistent after restart, before %v after %v", beforeRestart, afterRestart)
	}
	if err := verifyIPExist(afterRestart); err != nil {
		t.Fatal(err)
	}
}

func verifyIPExist(routes []string) error {
	expected, err := utils.GetMetadata("network-interfaces/0/ip-aliases/0")
	if err != nil {
		return fmt.Errorf("couldn't get first alias IP from metadata: %v", err)
	}
	for _, route := range routes {
		if route == expected {
			return nil
		}
	}
	return fmt.Errorf("alias ip %s is not exist after reboot", expected)
}

func compare(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
