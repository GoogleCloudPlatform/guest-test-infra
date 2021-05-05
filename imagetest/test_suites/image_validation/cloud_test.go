package imagevalidation

import (
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestNTPService Verify that ntp package exist and configuration is correct.
func TestNTPService(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata")
	}

	bytes, err := ioutil.ReadFile("/etc/ntp.conf")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(bytes), "\n")

	// The logic here expects at least one 'server' line in /etc/ntp.conf,
	// where the first 'server' line points to our metadata server, but without
	// caring where any subsequent server lines point. For example, Ubuntu uses
	// metadata.google.internal in the first server line and ntp.ubuntu.com on
	// the second.
	for _, line := range lines {
		if strings.HasPrefix(line, "server") {
			// Usually the line is like "server serverName url"
			serverName := strings.Split(line, " ")[1]
			if !hasCorrectMetadataServer(serverName) {
				t.Fatalf("ntp.conf contains wrong server information %s'", line)
			}
			break
		}
	}

	cmd := exec.Command("systemctl", "status", "ntpd")
	bytes, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	out := string(bytes)

	if strings.Contains(image, "rhel") {
		return
	}

	// Make sure that ntpd service is running.
	if !strings.Contains(out, "active (running)") {
		t.Fatalf("ntpd process is of wrong state %s", out)
	}
}

func hasCorrectMetadataServer(serverName string) bool {
	return serverName == "metadata.google.internal" || serverName == "metadata" || serverName == "169.254.169.254"
}
