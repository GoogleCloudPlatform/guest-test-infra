// +build cit

package imageboot

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// The values have been decided based on running spot tests for different images.
var imageFamilyBootTimeThresholdMap = map[string]int{
	"almalinux":   60,
	"centos":      60,
	"debian":      50,
	"rhel":        60,
	"rocky-linux": 60,
	"sles-12":     85,
	"sles-15":     120,
	"ubuntu":      75,
	"ubuntu-pro":  110,
}

const (
	markerFile     = "/boot-marker"
	secureBootFile = "/sys/firmware/efi/efivars/SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"
)

func lookForSshdAndGuestAgentProcess() error {
	dir, _ := os.Open("/proc")
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		return err
	}

	var foundSshd bool
	var foundGuestAgent bool

	for _, name := range names {
		// Continue if the directory name does start with a number
		if name[0] < '0' || name[0] > '9' {
			continue
		}

		// Continue if the directory name is not an integer
		_, err := strconv.ParseInt(name, 10, 0)
		if err != nil {
			continue
		}

		// Gets the symbolic link to the executable
		link, err := os.Readlink("/proc/" + name + "/exe")
		if err != nil {
			continue
		}

		if strings.Trim(link, "\n") == "/usr/sbin/sshd" {
			foundSshd = true
		}

		if strings.Trim(link, "\n") == "/usr/bin/google_guest_agent" {
			foundGuestAgent = true
		}

	}

	if foundSshd && foundGuestAgent {
		return nil
	} else {
		return fmt.Errorf("not found")
	}
}

func TestGuestBoot(t *testing.T) {
	t.Log("Guest booted successfully")
}

func TestGuestReboot(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	t.Log("marker file exist signal the guest reboot successful")
}

func TestGuestRebootOnHost(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("shutdown", "-r", "-t", "0")
		} else {
			cmd = exec.Command("sudo", "nohup", "reboot")
		}
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run reboot command: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	t.Log("marker file exist signal the guest reboot successful")
}

func TestGuestSecureBoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		if err := testWindowsGuestSecureBoot(); err != nil {
			t.Fatalf("SecureBoot test failed with: %v", err)
		}
	} else {
		image, err := utils.GetMetadata("image")
		if err != nil {
			t.Fatalf("couldn't get image from metadata")
		}
		if strings.Contains(image, "debian-9") {
			t.Skip("secure boot is not supported on Debian 9")
		}

		if err := testLinuxGuestSecureBoot(); err != nil {
			t.Fatalf("SecureBoot test failed with: %v", err)
		}
	}
}

func testLinuxGuestSecureBoot() error {
	if _, err := os.Stat(secureBootFile); os.IsNotExist(err) {
		return errors.New("efi var is missing")
	}
	data, err := ioutil.ReadFile(secureBootFile)
	if err != nil {
		return errors.New("failed reading secure boot file")
	}
	// https://www.kernel.org/doc/Documentation/ABI/stable/sysfs-firmware-efi-vars
	if data[len(data)-1] != 1 {
		return errors.New("secure boot is not enabled as expected")
	}

	return nil
}

func testWindowsGuestSecureBoot() error {
	cmd := exec.Command("powershell.exe", "Confirm-SecureBootUEFI")

	output, err := cmd.Output()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to run SecureBoot command: %v", err))
	}

	// The output will return a string that is either 'True' or 'False'
	// so we need to parse it and compare here.
	if trimmed_output := strings.TrimSpace(string(output)); trimmed_output != "True" {
		return errors.New("Secure boot is not enabled as expected")
	}

	return nil
}

func TestStartTime(t *testing.T) {
	metadata, err := utils.GetMetadataAttribute("start-time")
	if err != nil {
		t.Fatalf("couldn't get start time from metadata")
	}
	startTime, err := strconv.Atoi(metadata)
	if err != nil {
		t.Fatalf("failed to convet start time %s", metadata)
	}
	t.Logf("image start time is %d", time.Now().Second()-startTime)
}

func TestBootTime(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	var maxThreshold int

	switch {
	case strings.Contains(image, "almalinux"):
		maxThreshold = imageFamilyBootTimeThresholdMap["almalinux"]
	case strings.Contains(image, "centos"):
		maxThreshold = imageFamilyBootTimeThresholdMap["centos"]
	case strings.Contains(image, "debian"):
		maxThreshold = imageFamilyBootTimeThresholdMap["debian"]
	case strings.Contains(image, "rhel"):
		maxThreshold = imageFamilyBootTimeThresholdMap["rhel"]
	case strings.Contains(image, "rocky-linux"):
		maxThreshold = imageFamilyBootTimeThresholdMap["rocky-linux"]
	case strings.Contains(image, "sles-12"):
		maxThreshold = imageFamilyBootTimeThresholdMap["sles-12"]
	case strings.Contains(image, "sles-15"):
		maxThreshold = imageFamilyBootTimeThresholdMap["sles-15"]
	case strings.Contains(image, "ubuntu-pro"):
		maxThreshold = imageFamilyBootTimeThresholdMap["ubuntu-pro"]
	case strings.Contains(image, "ubuntu"):
		maxThreshold = imageFamilyBootTimeThresholdMap["ubuntu"]
	default:
		t.Fatalf("OS not supported for boot time test")
	}

	var foundGuestAgentAndSshd bool

	// 120 is the current maximum number of seconds to allow any distro to start sshd and guest-agent before returning a test failure
	for i := 0; i < 120; i++ {
		if err := lookForSshdAndGuestAgentProcess(); err == nil {
			foundGuestAgentAndSshd = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !foundGuestAgentAndSshd {
		t.Fatalf("Condition for guest agent and sshd process to start not reached within timeout")
	}

	// Reading the system uptime once both guest agent and sshd are found in the processes
	uptimeData, err := os.ReadFile("/proc/uptime")
	if err != nil {
		t.Fatalf("Failed to read uptime file")
	}
	fields := strings.Split(string(uptimeData), " ")
	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		t.Fatalf("Failed to read uptime numeric value")
	}
	t.Logf("found guest agent and sshd running at %v seconds", int(uptime))

	//Validating the uptime against the allowed threshold value
	if int(uptime) > maxThreshold {
		t.Errorf("Boot time too long: %v is beyond max of %v", uptime, maxThreshold)
	}
}
