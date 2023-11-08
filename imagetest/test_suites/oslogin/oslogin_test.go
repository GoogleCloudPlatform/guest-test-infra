//go:build cit
// +build cit

package oslogin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

//const testUsername = "sa_105020877179577573373"
//const testUUID = "3651018652"
const (
	testUsername = "sa_115308896887453382826"
	testUUID = "2260770985"
)

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

func isTwoFactorAuthEnabled(ctx context.Context) (bool, error) {
	var (
		instanceFlag, projectFlag bool
		err                       error
	)

	elem := []string{"attributes", "enable-oslogin-2fa"}

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

func isOsLoginEnabled(ctx context.Context) (error) {
	data, err := os.ReadFile("/etc/nsswitch.conf")
	if err != nil {
		return fmt.Errorf("cannot read /etc/nsswitch.conf")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "passwd:") && !strings.Contains(line, "oslogin") {
			return fmt.Errorf("OS Login not enabled in /etc/nsswitch.conf.")
		}
	}

	// Check AuthorizedKeys Command
	data, err = os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return fmt.Errorf("cannot read /etc/ssh/sshd_config")
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
		return fmt.Errorf("AuthorizedKeysCommand not set up for OS Login.")
	}

	if err = testSSHDPamConfig(ctx); err != nil {
		return fmt.Errorf("error checking pam config: %v", err)
	}
	return nil
}

func TestOsLoginEnabled(t *testing.T) {
	if err := isOsLoginEnabled(utils.Context(t)); err != nil {
		t.Fatalf(err.Error())
	}
}

func testSSHDPamConfig(ctx context.Context) error {
	twoFactorAuthEnabled, err := isTwoFactorAuthEnabled(ctx)
	if err != nil {
		return fmt.Errorf("Failed to query two factor authentication metadata entry: %+v", err)
	}

	if twoFactorAuthEnabled {
		// Check Pam Modules
		data, err := os.ReadFile("/etc/pam.d/sshd")
		if err != nil {
			return fmt.Errorf("cannot read /etc/pam.d/sshd: %+v", err)
		}

		if !strings.Contains(string(data), "pam_oslogin_login.so") {
			return fmt.Errorf("OS Login PAM module missing from pam.d/sshd.")
		}
	}
	return nil
}

func TestOsLoginDisabled(t *testing.T) {
	// Check OS Login not enabled in /etc/nsswitch.conf
	data, err := os.ReadFile("/etc/nsswitch.conf")
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
	data, err = os.ReadFile("/etc/ssh/sshd_config")
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

	if err = testSSHDPamConfig(utils.Context(t)); err != nil {
		t.Fatalf("error checking pam config: %v", err)
	}
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
