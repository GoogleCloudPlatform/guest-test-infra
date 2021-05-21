package security

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

var securitySettingMap = map[string]int{
	"net.ipv4.ip_forward":                        0,
	"net.ipv4.tcp_syncookies":                    1,
	"net.ipv4.conf.all.accept_source_route":      0,
	"net.ipv4.conf.default.accept_source_route":  0,
	"net.ipv4.conf.all.accept_redirects":         0,
	"net.ipv4.conf.default.accept_redirects":     0,
	"net.ipv4.conf.all.secure_redirects":         1,
	"net.ipv4.conf.default.secure_redirects":     1,
	"net.ipv4.conf.all.send_redirects":           0,
	"net.ipv4.conf.default.send_redirects":       0,
	"net.ipv4.conf.all.rp_filter":                1,
	"net.ipv4.conf.default.rp_filter":            1,
	"net.ipv4.icmp_echo_ignore_broadcasts":       1,
	"net.ipv4.icmp_ignore_bogus_error_responses": 1,
	"net.ipv4.conf.all.log_martians":             1,
	"net.ipv4.conf.default.log_martians":         1,
	"kernel.randomize_va_space":                  2,
}

var specialAccountShells = map[string]string{
	"sync":     "/bin/sync",
	"halt":     "/sbin/halt",
	"shutdown": "/sbin/shutdown",
}

const (
	unattendedUpgradeConfigPath  = "/etc/apt/apt.conf.d/50unattended-upgrades"
	unattendedUpgradeBlockDebian = `Unattended-Upgrade::Origins-Pattern`
	unattendedUpgradeBlockUbuntu = `Unattended-Upgrade::Allowed-Origins`
	expectedDebian               = `origin=Debian,codename=${distro_codename},label=Debian-Security`
	expectedUbuntu               = `${distro_id}:${distro_codename}-security`
	maxUID                       = 1000
	maxInterval                  = 7
	minInterval                  = 1
	sysctlBase                   = "/proc/sys/"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

// TestKernelSecuritySettings Checks that the given parameter has the given value in sysctl.
func TestKernelSecuritySettings(t *testing.T) {
	for key, expect := range securitySettingMap {
		v, err := sysctlGet(key)
		if err != nil {
			t.Fatalf("failed getting key %s", key)
		}
		value, err := strconv.Atoi(v)
		if err != nil {
			t.Fatalf("failed convert config %s value %s to int", key, v)
		}
		if expect != value {
			t.Fatalf("expected %s = %d. found %d.", key, expect, value)
		}
	}
}

// TestAutomaticUpdates Check automatic security updates are installed or enabled.
func TestAutomaticUpdates(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}
	if err := verifyServiceEnabled(image); err != nil {
		t.Fatal(err)
	}
	// Check that the security packages are marked for automatic update
	// https://wiki.debian.org/UnattendedUpgrades
	// https://help.ubuntu.com/community/AutomaticSecurityUpdates
	if err := verifySecurityUpgrade(image); err != nil {
		t.Fatal(err)
	}
	if err := verifyAutomaticUpdate(image); err != nil {
		t.Fatal(err)
	}
}

// TestPasswordSecurity Ensure that the system enforces strong passwords and correct lockouts.
func TestPasswordSecurity(t *testing.T) {
	if err := verifySSHConfig(); err != nil {
		t.Fatal(err)
	}

	// Root password/login is disabled.
	if err := verifyPassword(); err != nil {
		t.Fatal(err)
	}
}

func verifyPassword() error {
	image, err := utils.GetMetadata("image")
	if err != nil {
		return fmt.Errorf("couldn't get image from metadata")
	}
	fileBytes, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return err
	}
	users := strings.Split(string(fileBytes), "\n")
	for _, user := range users {
		// ignore empty user
		if len(user) == 0 {
			continue
		}
		passwdItems := strings.Split(user, ":")
		loginname := passwdItems[0]
		passwd := passwdItems[1]
		uid := passwdItems[2]
		shell := passwdItems[6]
		// A passwd entry of length 1 is a disabled password.
		if len(passwd) != 1 {
			return fmt.Errorf("login for %s does not look disabled", loginname)
		}

		uidValue, err := strconv.Atoi(uid)
		if err != nil {
			return err
		}
		// Don't check user account and root account
		if uidValue >= maxUID || uidValue == 0 {
			continue
		}
		if targetShell, found := specialAccountShells[loginname]; found {
			if targetShell != shell {
				return fmt.Errorf("account %s has wrong login shell %s", loginname, shell)
			}
		} else {
			// SUSE has bin user with login access
			if !strings.Contains(image, "sles") && !strings.Contains(shell, "false") && !strings.Contains(shell, "nologin") {
				return fmt.Errorf("account %s has the login shell %s", loginname, shell)
			}
		}
	}
	return nil
}

func verifySSHConfig() error {
	fileBytes, err := ioutil.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return err
	}
	sshdConfig := string(fileBytes)
	if !strings.Contains(sshdConfig, "PasswordAuthentication no") {
		return fmt.Errorf("\"PasswordAuthentication\" was not set to \"no\"")
	}

	noRootSSH := strings.Contains(sshdConfig, "PermitRootLogin no")
	noRootPasswordSSH := strings.Contains(sshdConfig, "PermitRootLogin without-password")
	if !noRootSSH && !noRootPasswordSSH {
		return fmt.Errorf("\"PermitRootLogin\" was not set to \"no\" or \"without-password\"")
	}
	return nil
}

func verifySecurityUpgrade(image string) error {
	var expectedBlock, expectedLine string
	if isRhelbasedLinux(image) {
		return nil
	}
	switch {
	case strings.Contains(image, "debian"):
		expectedBlock = unattendedUpgradeBlockDebian
		expectedLine = expectedDebian
	case strings.Contains(image, "ubuntu"):
		expectedBlock = unattendedUpgradeBlockUbuntu
		expectedLine = expectedUbuntu
	default:
		return fmt.Errorf("unsupport image %s", image)
	}
	// First verify package installed
	stdout, _, err := runCommand("dpkg-query", "-W", "--showformat", "${Status}", "unattended-upgrades")
	if err != nil {
		return err
	}
	if strings.TrimSpace(stdout) != "install ok installed" {
		return fmt.Errorf("package is not correctly installed")
	}
	// Second verify config is correct
	bytes, err := ioutil.ReadFile(unattendedUpgradeConfigPath)
	if err != nil {
		return err
	}
	var inBlock bool
	for _, line := range strings.Split(string(bytes), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.Contains(line, "//"):
			continue
		case strings.Contains(line, expectedBlock):
			inBlock = true
		case inBlock && strings.Contains(line, expectedLine):
			// Success!
			return nil
		case inBlock && strings.Contains(line, "};"):
			return fmt.Errorf("incorrect Unattended-Upgrade config")
		}
	}
	return fmt.Errorf("missing Unattended-Upgrade config")
}

func isRhelbasedLinux(image string) bool {
	return strings.Contains(image, "centos") || strings.Contains(image, "rhel") ||
			strings.Contains(image, "rocky-linux") || strings.Contains(image, "almalinux") ||
			strings.Contains(image, "sles")
}

func verifyServiceEnabled(image string) error {
	var serviceName string
	switch {
	case strings.Contains(image, "debian"), strings.Contains(image, "ubuntu"), strings.Contains(image, "sles"):
		return nil
	case strings.Contains(image, "rhel-7") || strings.Contains(image, "centos-7"):
		serviceName = "yum-cron"
	default:
		serviceName = "dnf-automatic.timer"
	}
	if _, _, err := runCommand("systemctl", "is-enabled", serviceName); err != nil {
		return err
	}
	return nil
}

func verifyAutomaticUpdate(image string) error {
	if isRhelbasedLinux(image) {
		return nil
	}

	automaticUpdateConfig, err := readAPTConfig(image)
	if err != nil {
		return err
	}
	switch {
	case strings.Contains(image, "debian"):
		if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Enable "1";`) {
			return fmt.Errorf(`"APT::Periodic::Enable" is not set to 1`)
		}
	case strings.Contains(image, "ubuntu"):
		// Ensure that we clean out obsolete debs within 7 days so that customer VMs
		// don't leak disk space. The value below is in days, with 0 as
		// disabled.
		re, err := regexp.Compile(`APT::Periodic::AutocleanInterval "(\d+)";`)
		if err != nil {
			return err
		}
		found := re.FindStringSubmatch(automaticUpdateConfig)
		if len(found) == 0 {
			return fmt.Errorf("no autoclean interval was specified")
		}
		interval, err := strconv.Atoi(found[1])
		if err != nil {
			return err
		}
		if interval > maxInterval || interval < minInterval {
			return fmt.Errorf("autoclean interval is invalid or an unexpected length")
		}
	default:
		return fmt.Errorf("unsupport image %s", image)
	}
	if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Update-Package-Lists "1";`) {
		return fmt.Errorf(`"APT::Periodic::Update-Package-Lists" is not set to 1`)
	}
	if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Unattended-Upgrade "1";`) {
		return fmt.Errorf(`"APT::Periodic::Unattended" is not set to 1`)
	}
	return nil
}

func readAPTConfig(image string) (string, error) {
	var configPaths []string
	var b []byte
	configPaths = append(configPaths, "/etc/apt/apt.conf.d/20auto-upgrades")
	if strings.Contains(image, "debian-9") {
		configPaths = append(configPaths, "/etc/apt/apt.conf.d/02periodic")
	} else if strings.Contains(image, "ubuntu") {
		configPaths = append(configPaths, "/etc/apt/apt.conf.d/10periodic")
	}
	for _, path := range configPaths {
		newByte, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}
		b = append(b, newByte...)
	}
	return string(b), nil
}

// sysctlGet returns a sysctl from a given key.
func sysctlGet(key string) (string, error) {
	path := filepath.Join(sysctlBase, strings.Replace(key, ".", "/", -1))
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func runCommand(name string, arg ...string) (string, string, error) {
	cmd := exec.Command(name, arg...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Start(); err != nil {
		return outBuf.String(), errBuf.String(), fmt.Errorf("failed starting cmd %s with error %v", cmd.String(), err)
	}
	if err := cmd.Wait(); err != nil {
		return outBuf.String(), errBuf.String(), fmt.Errorf("failed waiting cmd %s with error %v", cmd.String(), err)
	}
	return outBuf.String(), errBuf.String(), nil
}
