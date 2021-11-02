//go:build cit
// +build cit

package oslogin

import (
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"
)

const TEST_USERNAME = "sa_105020877179577573373"
const TEST_UID = "3651018652"
const TEST_USER_ENTRY = TEST_USERNAME + ":*:" + TEST_UID + ":" + TEST_UID + "::/home/" + TEST_USERNAME

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
	var found bool
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "AuthorizedKeysCommand") && strings.Contains(line, "/usr/bin/google_authorized_keys") {
			found = true
		}
	}
	if !found {
		t.Errorf("AuthorizedKeysCommand not set up for OS Login.")
	}

	// Check Pam Modules
	data, err = ioutil.ReadFile("/etc/pam.d/sshd")
	if err != nil {
		t.Fatalf("cannot read /etc/pam.d/sshd")
	}
	contents := string(data)
	if !strings.Contains(contents, "pam_oslogin_login.so") || !strings.Contains(contents, "pam_oslogin_admin.so") {
		t.Errorf("OS Login PAM module missing from pam.d/sshd.")
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

	// Check Pam Modules
	data, err = ioutil.ReadFile("/etc/pam.d/sshd")
	if err != nil {
		t.Fatalf("cannot read /etc/pam.d/sshd")
	}
	contents := string(data)
	if strings.Contains(contents, "pam_oslogin_login.so") || strings.Contains(contents, "pam_oslogin_admin.so") {
		t.Errorf("OS Login PAM module wrongly included in pam.d/sshd when disabled.")
	}
}

func TestGetentPasswdOsloginUser(t *testing.T) {
	out := RunGetent(t, "passwd", TEST_USERNAME)
	if !strings.Contains(out, TEST_USER_ENTRY) {
		t.Errorf("getent passwd output does not contain %s", TEST_USER_ENTRY)
	}
}

func TestGetentPasswdAllUsers(t *testing.T) {
	out := RunGetent(t, "passwd")
	if !strings.Contains(out, "root:x:0:0:root:/root:") {
		t.Errorf("getent passwd output does not contain user root")
	}
	if !strings.Contains(out, "nobody:x:") {
		t.Errorf("getent passwd output does not contain user nobody")
	}
	if !strings.Contains(out, TEST_USER_ENTRY) {
		t.Errorf("getent passwd output does not contain %s", TEST_USER_ENTRY)
	}
}

func TestGetentPasswdOsloginUID(t *testing.T) {
	out := RunGetent(t, "passwd", TEST_UID)
	if !strings.Contains(out, TEST_USER_ENTRY) {
		t.Errorf("getent passwd output does not contain %s", TEST_USER_ENTRY)
	}
}

func TestGetentPasswdLocalUser(t *testing.T) {
	out := RunGetent(t, "passwd", "nobody")
	if !strings.Contains(out, "nobody:x:") {
		t.Errorf("getent passwd output does not contain user nobody")
	}
}

func TestGetentPasswdInvalidUser(t *testing.T) {
	err := RunGetentErr(t, "passwd", "__invalid_user___")
	if err.Error() != "exit status 2" {
		t.Errorf("getent passwd did not give error on invaid user")
	}
}

func RunGetent(t *testing.T, args ...string) string {
	cmd := exec.Command("/usr/bin/getent", args...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("getent command failed %v", err)
	}
	return string(out)
}

func RunGetentErr(t *testing.T, args ...string) error {
	cmd := exec.Command("/usr/bin/getent", args...)
	err := cmd.Run()
	return err
}
