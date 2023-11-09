//go:build cit
// +build cit

package oslogin

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const testUsername = "sa_105020877179577573373"
const testUUID = "3651018652"

var testUserEntry = fmt.Sprintf("%s:*:%s:%s::/home/%s:", testUsername, testUUID, testUUID, testUsername)

func getTwoFactorAuth(ctx context.Context, root string, elem ...string) (bool, error) {
	data, err := utils.GetMetadata(ctx, append([]string{root}, elem...)...)
	if err != nil {
		return false, err
	}

	flag, err := strconv.ParseBool(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse enable-oslogin-2fa metadata entry: %+v", err)
	}

	return flag, nil
}

func isTwoFactorAuthEnabled(t *testing.T) (bool, error) {
	var (
		instanceFlag, projectFlag bool
		err                       error
	)

	elem := []string{"attributes", "enable-oslogin-2fa"}
	ctx := utils.Context(t)

	instanceFlag, err = getTwoFactorAuth(ctx, "instance", elem...)
	if err != nil && !errors.Is(err, utils.ErrMDSEntryNotFound) {
		return false, err
	}

	projectFlag, err = getTwoFactorAuth(ctx, "project", elem...)
	if err != nil && !errors.Is(err, utils.ErrMDSEntryNotFound) {
		return false, err
	}

	return instanceFlag || projectFlag, nil
}

func TestOsLoginEnabled(t *testing.T) {
	// Check OS Login enabled in /etc/nsswitch.conf
	data, err := ioutil.ReadFile("/etc/nsswitch.conf")
	if err != nil {
		t.Fatalf("cannot read /etc/nsswitch.conf")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "passwd:") && !strings.Contains(line, "oslogin") {
			t.Errorf("OS Login not enabled in /etc/nsswitch.conf.")
		}
	}

	// Check AuthorizedKeys Command
	data, err = ioutil.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		t.Fatalf("cannot read /etc/ssh/sshd_config")
	}
	var foundAuthorizedKeys bool
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "AuthorizedKeysCommand") && strings.Contains(line, "/usr/bin/google_authorized_keys") {
			foundAuthorizedKeys = true
		}
	}

	if !foundAuthorizedKeys {
		t.Errorf("AuthorizedKeysCommand not set up for OS Login.")
	}

	testSSHDPamConfig(t)
}

func testSSHDPamConfig(t *testing.T) {
	t.Helper()

	twoFactorAuthEnabled, err := isTwoFactorAuthEnabled(t)
	if err != nil {
		t.Fatalf("Failed to query two factor authentication metadata entry: %+v", err)
	}

	if twoFactorAuthEnabled {
		// Check Pam Modules
		data, err := ioutil.ReadFile("/etc/pam.d/sshd")
		if err != nil {
			t.Fatalf("cannot read /etc/pam.d/sshd: %+v", err)
		}

		if !strings.Contains(string(data), "pam_oslogin_login.so") {
			t.Errorf("OS Login PAM module missing from pam.d/sshd.")
		}
	}
}

func TestOsLoginDisabled(t *testing.T) {
	// Check OS Login not enabled in /etc/nsswitch.conf
	data, err := ioutil.ReadFile("/etc/nsswitch.conf")
	if err != nil {
		t.Fatalf("cannot read /etc/nsswitch.conf")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "passwd:") && strings.Contains(line, "oslogin") {
			t.Errorf("OS Login NSS module wrongly included in /etc/nsswitch.conf when disabled.")
		}
	}

	// Check AuthorizedKeys Command
	data, err = ioutil.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		t.Fatalf("cannot read /etc/ssh/sshd_config")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "AuthorizedKeysCommand") && strings.Contains(line, "/usr/bin/google_authorized_keys") {
			t.Errorf("OS Login AuthorizedKeysCommand directive wrongly exists when disabled.")
		}
	}

	testSSHDPamConfig(t)
}

func TestGetentPasswdOsloginUser(t *testing.T) {
	cmd := exec.Command("getent", "passwd", testUsername)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("getent command failed %v", err)
	}
	if !strings.Contains(string(out), testUserEntry) {
		t.Errorf("getent passwd output does not contain %s", testUserEntry)
	}
}

func TestGetentPasswdAllUsers(t *testing.T) {
	cmd := exec.Command("getent", "passwd")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("getent command failed %v", err)
	}
	if !strings.Contains(string(out), "root:x:0:0:root:/root:") {
		t.Errorf("getent passwd output does not contain user root")
	}
	if !strings.Contains(string(out), "nobody:x:") {
		t.Errorf("getent passwd output does not contain user nobody")
	}
	if !strings.Contains(string(out), testUserEntry) {
		t.Errorf("getent passwd output does not contain %s", testUserEntry)
	}
}

func TestGetentPasswdOsloginUID(t *testing.T) {
	cmd := exec.Command("getent", "passwd", testUUID)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("getent command failed %v", err)
	}
	if !strings.Contains(string(out), testUserEntry) {
		t.Errorf("getent passwd output does not contain %s", testUserEntry)
	}
}

func TestGetentPasswdLocalUser(t *testing.T) {
	cmd := exec.Command("getent", "passwd", "nobody")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("getent command failed %v", err)
	}
	if !strings.Contains(string(out), "nobody:x:") {
		t.Errorf("getent passwd output does not contain user nobody")
	}
}

func TestGetentPasswdInvalidUser(t *testing.T) {
	cmd := exec.Command("getent", "passwd", "__invalid_user__")
	err := cmd.Run()
	if err.Error() != "exit status 2" {
		t.Errorf("getent passwd did not give error on invalid user")
	}
}
