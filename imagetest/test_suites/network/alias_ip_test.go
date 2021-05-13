package network

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	markerFile  = "/boot-marker"
	aliasipFile = "/aliasip"
)

func TestAliasAfterReboot(t *testing.T) {
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

	_, err = os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		b, err := getAliasIP(networkInterface)
		if err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(aliasipFile, b, 0644); err != nil {
			t.Fatal("failed writing alias ip to file")
		}
	}
	// second boot
	before, err := getAliasIP(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	after, err := ioutil.ReadFile(aliasipFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("routes are inconsistent after reboot, before %s, after %s", before, after)
	}
}

func getAliasIP(networkInterface string) ([]byte, error) {
	arguments := strings.Split(fmt.Sprintf("route list table local type local scope host dev %s proto 66", networkInterface), " ")
	cmd := exec.Command("ip", arguments...)
	b, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("alias IPs not configured")
	}
	return b, nil
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

	beforeRestart, err := getAliasIP(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("systemctl", "restart", "google-guest-agent")
	_, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	afterRestart, err := getAliasIP(networkInterface)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeRestart) != string(afterRestart) {
		t.Fatalf("routes are inconsistent after reboot, before %s after %s", beforeRestart, afterRestart)
	}
}
