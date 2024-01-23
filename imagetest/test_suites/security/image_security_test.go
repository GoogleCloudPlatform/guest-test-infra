//go:build cit
// +build cit

package security

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
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

// TestKernelSecuritySettings Checks that the given parameter has the given value in sysctl.
func TestKernelSecuritySettings(t *testing.T) {
	utils.LinuxOnly(t)
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
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "debian"):
		if err := verifySecurityUpgrade(image); err != nil {
			t.Fatal(err)
		}
		if err := verifyAutomaticUpdate(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "ubuntu"):
		if err := verifySecurityUpgrade(image); err != nil {
			t.Fatal(err)
		}
		if err := verifyAutomaticUpdate(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "windows"):
		if err := verifyAutomaticUpdate(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "suse"):
		t.Skip("Not supported on SUSE")
	case strings.Contains(image, "sles"):
		t.Skip("Not supported on SLES")
	case strings.Contains(image, "fedora"):
		t.Skip("Not supported on Fedora")
	case strings.Contains(image, "centos"):
		if err := verifyServiceEnabled(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "rhel"):
		if err := verifyServiceEnabled(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "almalinux"):
		if err := verifyServiceEnabled(image); err != nil {
			t.Fatal(err)
		}
	case strings.Contains(image, "rocky-linux"):
		if err := verifyServiceEnabled(image); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("image %s not support", image)
	}
}

// TestPasswordSecurity Ensure that the system enforces strong passwords and correct lockouts.
func TestPasswordSecurity(t *testing.T) {
	ctx := utils.Context(t)
	image, err := utils.GetMetadata(ctx, "instance", "image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	if err := verifySSHConfig(t, image); err != nil {
		t.Fatal(err)
	}
	if utils.IsWindows() {
		return
	}

	// Root password/login is disabled.
	if err := verifyPassword(ctx); err != nil {
		t.Fatal(err)
	}
}

func verifyPassword(ctx context.Context) error {
	image, err := utils.GetMetadata(ctx, "instance", "image")
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

func verifySSHConfig(t *testing.T, image string) error {
	t.Helper()
	var sshdConfig []byte
	var err error
	if utils.IsWindows() {
		addUser := exec.CommandContext(utils.Context(t), `net`, `user`, `testadmin`, `password123!`, `/add`)
		o, err := addUser.Output()
		if err != nil {
			t.Fatalf("failed to add testadmin user: %v; output: %s", err, o)
		}
		addToGrp := exec.CommandContext(utils.Context(t), `net`, `localgroup`, `administrators`, `testadmin`, `/add`)
		err = addToGrp.Run()
		if err != nil {
			t.Fatalf("failed to add testadmin to administrators: %v; output: %s", err, o)
		}
		sshdConfig, err = exec.CommandContext(utils.Context(t), `C:\Program Files\OpenSSH\sshd.exe`, `-C`, `user=testadmin`, "-T").Output()
	} else {
		sshdConfig, err = exec.CommandContext(utils.Context(t), "sshd", "-T").Output()
	}
	if err != nil {
		return fmt.Errorf("could not get effective sshd config: %s", err)
	}
	// Effective configuration keys are all lowercase
	passwordauthsetting := strings.TrimSuffix(strings.TrimSuffix(string(regexp.MustCompile(`passwordauthentication[ \t]+[a-zA-Z]+\r?\n`).Find(sshdConfig)), "\n"), "\r")
	if passwordauthsetting != "passwordauthentication no" {
		return fmt.Errorf("sshd passwordauthentication setting is %q, want %q", passwordauthsetting, "passwordauthencation no")
	}
	if strings.Contains(image, "sles") || strings.Contains(image, "suse") || utils.IsWindows() {
		// SLES ships with "PermitRootLogin yes" in SSHD config.
		// This setting is meaningless on windows
		return nil
	}

	permitrootloginsetting := strings.TrimSuffix(strings.TrimSuffix(string(regexp.MustCompile(`permitrootlogin[ \t]+[a-zA-Z\-]+\r?\n`).Find(sshdConfig)), "\n"), "\r")
	if permitrootloginsetting != "permitrootlogin no" && permitrootloginsetting != "permitrootlogin prohibit-password" && permitrootloginsetting != "permitrootlogin without-password" {
		return fmt.Errorf("sshd permitrootlogin setting is %q, want %q, %q, or %q", permitrootloginsetting, "permitrootlogin no", "permitrootlogin prohibit-password", "permitrootlogin without-password")
	}
	return nil
}

// verifySecurityUpgrade Check that the security packages are marked for automatic update.
// https://wiki.debian.org/UnattendedUpgrades
// https://help.ubuntu.com/community/AutomaticSecurityUpdates
func verifySecurityUpgrade(image string) error {
	var expectedBlock, expectedLine string
	switch {
	case strings.Contains(image, "debian"):
		expectedBlock = unattendedUpgradeBlockDebian
		expectedLine = expectedDebian
	case strings.Contains(image, "ubuntu"):
		expectedBlock = unattendedUpgradeBlockUbuntu
		expectedLine = expectedUbuntu
	default:
		return fmt.Errorf("unsupported image %s", image)
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

func verifyServiceEnabled(image string) error {
	var serviceName string
	switch {
	case strings.Contains(image, "rhel-7") || strings.Contains(image, "centos-7"):
		serviceName = "yum-cron"
	default:
		serviceName = "dnf-automatic.timer"
	}
	_, _, err := runCommand("systemctl", "is-enabled", serviceName)
	return err
}

func verifyAutomaticUpdate(image string) error {
	if strings.Contains(image, "windows") {
		AUOptions, err := utils.RunPowershellCmd(`Get-ItemProperty -Path HKLM:\software\policies\microsoft\windows\windowsupdate\au | Format-List -Property AUOptions`)
		if err != nil {
			return err
		}
		if !strings.Contains(AUOptions.Stdout, "AUOptions : 4") {
			return fmt.Errorf("Unexpected AUOptions, got %q want %q", AUOptions.Stdout, "AUOptions : 4")
		}
		return nil
	}
	output, err := exec.Command("apt-config", "dump").Output()
	if err != nil {
		return err
	}
	automaticUpdateConfig := string(output)
	switch {
	case strings.Contains(image, "debian-9"):
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
			return fmt.Errorf("autoclean interval is %d, want greater or equal to %d and less or equal to %d", interval, minInterval, maxInterval)
		}
	}
	if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Update-Package-Lists "1";`) {
		return fmt.Errorf(`"APT::Periodic::Update-Package-Lists" is not set to 1`)
	}
	if !strings.Contains(automaticUpdateConfig, `APT::Periodic::Unattended-Upgrade "1";`) {
		return fmt.Errorf(`"APT::Periodic::Unattended" is not set to 1`)
	}
	return nil
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

var (
	allowedTCP = []string{
		"22",   // ssh
		"5355", // systemd-resolve
	}
	allowedUDP = []string{
		"68",   // bootpc aka DHCP client port
		"123",  // ntp
		"546",  // dhcp v6 client port
		"5355", // systemd-resolve
	}
)

// TestSockets tests that only whitelisted ports are listening globally.
func TestSockets(t *testing.T) {
	if utils.IsWindows() {
		validateSocketsWindows(t)
		return
	}
	// print listening TCP or UDP sockets with no header and no name
	// resolution.
	out, err := exec.Command("ss", "-Hltun").Output()
	if err != nil && err.Error() == "exit status 255" {
		// Probably on an OS with a version of ss too old to support -H
		out, err = exec.Command("ss", "-ltun").Output()
		_, a, _ := strings.Cut(string(out), "\n")
		out = []byte(a)
	}
	if err != nil {
		t.Fatalf("failed running ss command: %v", err)
	}
	var listenTCP []string
	var listenUDP []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 6 {
			t.Fatal("ss command format mismatch, should be 6-col output")
		}
		// 'Local Address:Port' column from ss output.
		listen := fields[4]

		switch {
		// Check explicitly for these formats as a safeguard, even
		// though this is all we requested with -l -t -u args.
		case fields[0] == "tcp" && fields[1] == "LISTEN":
			listenTCP = append(listenTCP, listen)
		case fields[0] == "udp" && fields[1] == "UNCONN":
			listenUDP = append(listenUDP, listen)
		default:
			t.Fatalf("ss command format mismatch %q", line)
		}
	}
	if len(listenTCP) == 0 && len(listenUDP) == 0 {
		// We should always have some listening sockets, such as for
		// SSH. If we didn't match any above, test logic is faulty.
		t.Fatalf("No listening sockets")
	}
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	if strings.Contains(image, "-sap") {
		// All SAP Images are permitted to have 'rpcbind' listening on
		// port 111
		allowedTCP = append(allowedTCP, "111")
		allowedUDP = append(allowedUDP, "111")
	}

	if !(strings.Contains(image, "rhel-7") && strings.Contains(image, "-sap")) {
		// Skip UDP check on RHEL-7-SAP images which have old rpcbind
		// which listens to random UDP ports.
		if err := validateSockets(listenUDP, allowedUDP); err != nil {
			t.Error(err)
		}
	}
	if err := validateSockets(listenTCP, allowedTCP); err != nil {
		t.Error(err)
	}
}

func validateSockets(listening, allowed []string) error {
	for _, entry := range listening {
		idx := strings.LastIndex(entry, ":")
		if idx == -1 {
			return fmt.Errorf("malformed listening address: %s", entry)
		}
		address := entry[:idx]
		port := entry[idx+1:]

		switch {
		case strings.HasPrefix(address, "127."):
			// IPv4 loopback addresses, not global.
			continue
		case address == "::1", address == "[::1]":
			// IPv6 localhost address, not global.
			continue
		case isInSlice(port, allowed):
			// Whitelisted global listening port.
			continue
		default:
			return fmt.Errorf("forbidden listening socket address %s port %s", address, port)
		}
	}

	return nil
}

func validateSocketsWindows(t *testing.T) {
	t.Helper()
	portRe := regexp.MustCompile("[0-9]+\r?\n")
	UDPListeners, err := utils.RunPowershellCmd(`Get-NetUDPEndpoint | Format-List -Property LocalPort`)
	if err != nil {
		t.Fatalf("could not get udp listeners: %v", err)
	}
	for _, port := range portRe.FindAllString(UDPListeners.Stdout, -1) {
		p, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(port, "\n"), "\r"))
		if err != nil {
			continue
		}
		switch p {
		case 123: // NTP
			continue
		case 137: // NetBIOS name service
			continue
		case 138: // NetBIOS
			continue
		case 500: // IPSec IKE
			continue
		case 546: // dhcp
			continue
		case 3389: // RDP
			continue
		case 3544: // Teredo
			continue
		case 4500: // IPSec NAT Traversal
			continue
		case 5353: // multicast dns
			continue
		case 5355: // LLMNR
			continue
		case 5050: // svchost
			continue
		}
		owningprocess, err := utils.RunPowershellCmd(fmt.Sprintf(`(Get-Process -Id "$((Get-NetUDPEndpoint -LocalPort %d).OwningProcess)").ProcessName`, p))
		if err != nil {
			t.Errorf("could not find process name listening on port %d", p)
		}
		pname := strings.TrimSpace(owningprocess.Stdout)
		if p > 49152 && pname == "svchost" {
			continue
		}
		t.Errorf("found udp listener on unexpected port %d (process name %s)", p, pname)
	}
	TCPListeners, err := utils.RunPowershellCmd(`Get-NetTCPConnection -State Listen | Format-List -Property LocalPort`)
	if err != nil {
		t.Fatalf("could not get tcp listeners: %v", err)
	}
	for _, port := range portRe.FindAllString(TCPListeners.Stdout, -1) {
		p, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(port, "\n"), "\r"))
		if err != nil {
			continue
		}
		switch p {
		case 22: // sshd
			continue
		case 135: // msrpc
			continue
		case 139: // NetBIOS
			continue
		case 445: // microsoft-ds
			continue
		case 3389: // rdp
			continue
		case 5985: // winrm
			continue
		case 5986: // winrm
			continue
		case 20201: // ops agent
			continue
		case 20202: // ops agent
			continue
		case 47001: // windows remote management
			continue
		}
		owningprocess, err := utils.RunPowershellCmd(fmt.Sprintf(`(Get-Process -Id "$((Get-TCPConnection -State Listen -LocalPort %d).OwningProcess)").ProcessName`, p))
		if err != nil {
			t.Errorf("could not find process name listening on port %d", p)
		}
		pname := strings.TrimSpace(owningprocess.Stdout)
		if p > 49152 && (pname == "svchost" || pname == "Idle") {
			continue
		}
		t.Errorf("found tcp listener on unexpected port %d (process %s)", p, pname)
	}
}

func isInSlice(entry string, list []string) bool {
	for _, listentry := range list {
		if entry == listentry {
			return true
		}
	}
	return false
}
