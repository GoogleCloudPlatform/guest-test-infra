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
	unattendedUpgradeConfigPath  = "/etc/apt/apt.conf.d/50unattended-upgrades"
	unattendedUpgradeRegexDebian = "Unattended-Upgrade::Origins-Pattern.*?};"
	unattendedUpgradeRegexUbuntu = "Unattended-Upgrade::Origins-Pattern.*?};"
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
		verifyServiceEnabled(t, "dnf-automatic.timer")
	case strings.Contains(image, "rhel-7"):
		verifyServiceEnabled(t, "yum-cron")
	case strings.Contains(image, "debian"):
		verifyPackageInstalled(t)
		// Check that the security packages are marked for automatic update
		verifySecurityUpgradeEnabled(t, unattendedUpgradeRegexDebian, expectedDebian)
	case strings.Contains(image, "ubuntu"):
		verifyPackageInstalled(t)
		// Check that the security packages are marked for automatic update
		verifySecurityUpgradeEnabled(t, unattendedUpgradeRegexUbuntu, expectedUbuntu)
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
		out, err := runCommand("yum", "list", "installed", "cracklib")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cracklib") {
			t.Fatal("Package cracklib is not installed.")
		}
		out, err = runCommand("yum", "list", "installed", "cracklib-dicts")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "cracklib-dicts") {
			t.Fatal("Package cracklib-dicts is not installed.")
		}
	}

	bytes, err := ioutil.ReadFile("/etc/ssh/sshd_config")
	sshdConfig := string(bytes)
	if strings.Contains(sshdConfig, "PasswordAuthentication no") {
		t.Fatal("PasswordAuthentication was not set to \"no\".")
	}

	noRootSSH := strings.Contains(sshdConfig, "PermitRootLogin no")
	noRootPasswordSSH := strings.Contains(sshdConfig, "PermitRootLogin without-password")
	if !noRootSSH && !noRootPasswordSSH {
		t.Fatal("PermitRootLogin was not set to \"no\" or \"without-password\".")
	}

	// Root password/login is disabled.
	bytes, err = ioutil.ReadFile("/etc/passwd")
	passwdContent := string(bytes)
	passwd := strings.Split(passwdContent, "\n")
	//passwd = filter(len, passwd)
	for _, line := range passwd {
		passwdItems := strings.Split(line, ":")
		loginname := passwdItems[0]
		passwd := passwdItems[1]
		uid := passwdItems[2]
		shell := passwdItems[6]
		// A passwd entry of length 1 is a disabled password.
		if len(passwd) != 1 {
			t.Fatalf("Login for %s does not look disabled", loginname)
		}

		uidValue, err := strconv.Atoi(uid)
		if err != nil {
			t.Fatal(err)
		}
		// Don't check user account
		if uidValue >= minUID || uidValue == 0 {
			continue
		}
		for user, shell := range specialAccountShells {
			if loginname == user {
				if specialAccountShells[loginname] != shell {
					t.Fatalf("Account %s has wrong login shell %s", loginname, shell)
				} else {
					break
				}
			}
		}
		if !strings.Contains(shell, "false") && !strings.Contains(shell, "nologin") {
			t.Fatalf("Account %s has the login shell %s", loginname, shell)
		}
	}
}

func verifySecurityUpgradeEnabled(t *testing.T, regexString string, expectedLine string) {
	bytes, err := ioutil.ReadFile(unattendedUpgradeConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	re, err := regexp.Compile(regexString)
	if err != nil {
		t.Fatalf("failed compiling regex %s", regexString)
	}
	enabledUpgrades := strings.Split(re.FindString(string(bytes)), "\n")

	for _, line := range enabledUpgrades {
		res, err := regexp.MatchString(expectedLine, line)
		if err != nil {
			t.Fatal(err)
		}
		if res {
			return
		}
	}
	t.Fatal("security upgrades not enabled")
}

func verifyPackageInstalled(t *testing.T) {
	out, err := runCommand("dpkg", "--get-selections", "unattended-upgrades")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "install") {
		t.Fatal("unattended-upgrades is not installed")
	}
}

func verifyServiceEnabled(t *testing.T, service string) {
	out, err := runCommand("systemctl", "isenabled", service)
	if err != nil {
		t.Fatal(err)
	}
	if out != "enabled" {
		t.Fatalf("%s is not enabled", service)
	}
}

func runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	bytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed running cmd %s", cmd.String())
	}
	return string(bytes), nil
}
