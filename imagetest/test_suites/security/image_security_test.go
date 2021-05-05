package security

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"github.com/lorenzosaino/go-sysctl"
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
	unattendedUpgradeConfigPath = "/etc/apt/apt.conf.d/50unattended-upgrades"
	// (?s) Dot match new line character
	unattendedUpgradeRegexDebian = `(?s)Unattended-Upgrade::Origins-Pattern.*?};`
	unattendedUpgradeRegexUbuntu = `(?s)Unattended-Upgrade::Allowed-Origins.*?};`
	expectedDebian               = `\s*"origin=Debian,codename=\${distro_codename},label=Debian-Security";.*`
	expectedUbuntu               = `\s*"\${distro_id}:\${distro_codename}-security";.*`
	minUID                       = 1000
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
	for key, value := range securitySettingMap {
		found, err := sysctl.Get(key)
		if err != nil {
			t.Fatal("failed getting ")
		}
		v, err := strconv.Atoi(found)
		if err != nil {
			t.Fatal("failed convert to int")
		}
		if value != v {
			t.Fatalf("expected %s = %d. Found %d.", key, value, v)
		}
	}
}

// TestAutomaticUpdates Check automatic security updates are installed and enabled.
func TestAutomaticUpdates(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "rhel-8"):
		if err := verifyServiceEnabled("dnf-automatic.timer"); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "rhel-7"):
		if err := verifyServiceEnabled("yum-cron"); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "debian") || strings.Contains(image, "ubuntu"):
		if err := verifyPackageInstalled(); err != nil {
			t.Fatal(err)
		}
		// Check that the security packages are marked for automatic update
		// https://wiki.debian.org/UnattendedUpgrades
		// https://help.ubuntu.com/community/AutomaticSecurityUpdates
		if err := verifySecurityUpgradeEnabled(image); err != nil {
			t.Fatal(err)
		}
		if err := verifyAutomaticUpdate(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "sles"):
		t.Skip("Skipping test on openSUSE.")
	}
}

// TestPasswordSecurity Ensure that the system enforces strong passwords and correct lockouts.
func TestPasswordSecurity(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}
	if strings.Contains(image, "rhel") {
		if err := verifyCracklibInstalled(); err != nil {
			t.Fatal(err)
		}
	}

	if err := verifySSHConfig(); err != nil {
		t.Fatal(err)
	}

	// Root password/login is disabled.
	if err := verifyPassword(); err != nil {
		t.Fatal(err)
	}
}

func verifyPassword() error {
	bytes, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return err
	}
	passwdContent := string(bytes)
	passwd := strings.Split(passwdContent, "\n")
	for _, line := range passwd {
		// ignore empty line
		if len(line) == 0 {
			continue
		}
		passwdItems := strings.Split(line, ":")
		loginname := passwdItems[0]
		passwd := passwdItems[1]
		uid := passwdItems[2]
		shell := passwdItems[6]
		// A passwd entry of length 1 is a disabled password.
		if len(passwd) != 1 {
			return fmt.Errorf("Login for %s does not look disabled", loginname)
		}

		uidValue, err := strconv.Atoi(uid)
		if err != nil {
			return err
		}
		// Don't check user account
		if uidValue >= minUID || uidValue == 0 {
			continue
		}
		var isSpecialAccount = false
		for user, shell := range specialAccountShells {
			if loginname == user {
				isSpecialAccount = true
				if specialAccountShells[loginname] != shell {
					return fmt.Errorf("Account %s has wrong login shell %s", loginname, shell)
				} else {
					break
				}
			}
		}
		if !isSpecialAccount && !strings.Contains(shell, "false") && !strings.Contains(shell, "nologin") {
			return fmt.Errorf("Account %s has the login shell %s", loginname, shell)
		}
	}
	return nil
}

func verifySSHConfig() error {
	bytes, err := ioutil.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return err
	}
	sshdConfig := string(bytes)
	if !strings.Contains(sshdConfig, "PasswordAuthentication no") {
		return fmt.Errorf("PasswordAuthentication was not set to \"no\".")
	}

	noRootSSH := strings.Contains(sshdConfig, "PermitRootLogin no")
	noRootPasswordSSH := strings.Contains(sshdConfig, "PermitRootLogin without-password")
	if !noRootSSH && !noRootPasswordSSH {
		return fmt.Errorf("PermitRootLogin was not set to \"no\" or \"without-password\".")
	}
	return nil
}

func verifyCracklibInstalled() error {
	out, err := runCommand("yum", "list", "installed", "cracklib")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "cracklib") {
		return fmt.Errorf("Package cracklib is not installed.")
	}
	out, err = runCommand("yum", "list", "installed", "cracklib-dicts")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "cracklib-dicts") {
		return fmt.Errorf("Package cracklib-dicts is not installed.")
	}
	return nil
}

func verifySecurityUpgradeEnabled(image string) error {
	var regexString, expectedLine string
	if strings.Contains(image, "debian") {
		regexString = unattendedUpgradeRegexDebian
		expectedLine = expectedDebian
	} else if strings.Contains(image, "ubuntu") {
		regexString = unattendedUpgradeRegexUbuntu
		expectedLine = expectedUbuntu
	}
	bytes, err := ioutil.ReadFile(unattendedUpgradeConfigPath)
	if err != nil {
		return err
	}
	re, err := regexp.Compile(regexString)
	if err != nil {
		return err
	}
	enabledUpgrades := strings.Split(re.FindString(string(bytes)), "\n")

	for _, line := range enabledUpgrades {
		res, err := regexp.MatchString(expectedLine, line)
		if err != nil {
			return err
		}
		if res {
			return nil
		}
	}
	return fmt.Errorf("security upgrades is not enabled")
}

func verifyPackageInstalled() error {
	out, err := runCommand("dpkg", "--get-selections", "unattended-upgrades")
	if err != nil {
		return err
	}
	if !strings.Contains(out, "install") {
		return fmt.Errorf("unattended-upgrades is not installed")
	}
	return nil
}

func verifyServiceEnabled(service string) error {
	out, err := runCommand("systemctl", "is-enabled", service)
	if err != nil {
		return err
	}
	out = strings.Trim(out, "\n")
	if out != "enabled" {
		return fmt.Errorf("%s is not enabled", service)
	}
	return nil
}

func verifyAutomaticUpdate(image string) error {
	automaticUpdateConfig, err := readAPTConfig(image)
	if err != nil {
		return err
	}

	if strings.Contains(image, "debian-9") {
		if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Enable "1";`) {
			return fmt.Errorf("APT::Periodic::Enable  is not set to 1")
		}
	}
	if strings.Contains(image, "ubuntu") {
		// Ensure that we clean out obsolete debs within 7 days so that customer VMs
		// don't leak disk space. The value below is in days, with 0 as
		// disabled.
		re := regexp.MustCompile(`APT::Periodic::AutocleanInterval "(\d+)";`)
		found := re.FindString(automaticUpdateConfig)
		if found == "" {
			return fmt.Errorf("No autoclean interval was specified.")
		}
		intervalStr := strings.Trim(strings.Split(found, " ")[1], "\"")
		interval, err := strconv.Atoi(intervalStr)
		if err != nil {
			return err
		}
		if interval > 9 || interval < 1 {
			return fmt.Errorf("Autoclean interval is invalid or an unexpected length")
		}
	}
	if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Update-Package-Lists "1";`) {
		return fmt.Errorf("APT::Periodic::Update-Package-Lists is not set to 1")
	}
	if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Unattended-Upgrade "1";`) {
		return fmt.Errorf("APT::Periodic::Unattended- is not set to 1")
	}
	return nil
}

func readAPTConfig(image string) (string, error) {
	var configPaths []string
	var bytes []byte
	if strings.Contains(image, "debian-9") {
		configPaths = []string{"/etc/apt/apt.conf.d/02periodic"}
	} else if strings.Contains(image, "debian-10") {
		configPaths = []string{"/etc/apt/apt.conf.d/20auto-upgrades"}
	} else if strings.Contains(image, "ubuntu") {
		configPaths = []string{"/etc/apt/apt.conf.d/10periodic", "/etc/apt/apt.conf.d/20auto-upgrades"}
	}
	for _, path := range configPaths {
		newByte, err := ioutil.ReadFile(path)
		if err != nil {
			return "", err
		}
		bytes = append(bytes, newByte...)
	}
	return string(bytes), nil
}

func runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	bytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed running cmd %s", cmd.String())
	}
	return string(bytes), nil
}
