package imagevalidation

import (
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	chronyService = "chronyd"
	ntpdService   = "ntp"
)

var ntpConfig = []string{"/etc/ntp.conf"}
var chronyConfig = []string{"/etc/chrony.conf", "/etc/chrony/chrony.conf", "/etc/chrony.d/gce.conf"}

// TestNTPService Verify that ntp package exist and configuration is correct.
// For SUSE, the ntp service is ntpd. For others, the ntp service is chronyd
func TestNTPService(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata")
	}
	var servicename string
	var configPaths []string
	switch {
	case strings.Contains(image, "debian-10"):
	case strings.Contains(image, "debian-11"):
	case strings.Contains(image, "rhel"):
	case strings.Contains(image, "centos"):
	case strings.Contains(image, "sles"):
		servicename = chronyService
		configPaths = chronyConfig
	case strings.Contains(image, "ubuntu"):
		switch {
		case strings.Contains(image, "ubuntu-1604"):
		case strings.Contains(image, "ubuntu-minimal-1604"):
			servicename = ntpdService
			configPaths = ntpConfig
		default:
			servicename = chronyService
			configPaths = chronyConfig
		}
	default:
		servicename = ntpdService
		configPaths = ntpConfig
	}

	configContent, err := readNTPConfig(configPaths)
	if err != nil {
		t.Fatalf("failed reading ntp config file %s", err)
	}
	ntpConfigs := strings.Split(configContent, "\n")

	// The logic here expects at least one 'server' line in /etc/ntp.conf or
	// /etc/chrony.conf where the first 'server' line points to our metadata
	// server, but without caring where any subsequent server lines point.
	// For example, Ubuntu uses metadata.google.internal in the first server line
	// and ntp.ubuntu.com on the second.
	for _, config := range ntpConfigs {
		if strings.HasPrefix(config, "server") {
			// Usually the line is like "server serverName url"
			serverName := strings.Split(config, " ")[1]
			if !(serverName == "metadata.google.internal" || serverName == "metadata" || serverName == "169.254.169.254") {
				t.Fatalf("ntp.conf contains wrong server information %s'", config)
			}
			break
		}
	}

	cmd := exec.Command("systemctl", "status", servicename)
	bytes, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	out := string(bytes)

	// Make sure that ntp service is running.
	if !strings.Contains(out, "active (running)") {
		t.Fatalf("%s service is of wrong state %s", servicename, out)
	}
}

func readNTPConfig(configPaths []string) (string, error) {
	var totalBytes []byte
	for _, path := range configPaths {
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			continue
		}
		totalBytes = append(totalBytes, bytes...)
	}
	return string(totalBytes), nil
}
