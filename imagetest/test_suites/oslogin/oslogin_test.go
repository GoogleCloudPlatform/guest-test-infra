//go:build cit
// +build cit

package oslogin

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"
)

const testUsername = "sa_105020877179577573373"
const testUUID = "3651018652"

var testUserEntry = fmt.Sprintf("%s:*:%s:%s::/home/%s:", testUsername, testUUID, testUUID, testUsername)

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

	// Check Pam Modules
	data, err = ioutil.ReadFile("/etc/pam.d/sshd")
	if err != nil {
		t.Fatalf("cannot read /etc/pam.d/sshd")
	}
	contents := string(data)
	if !strings.Contains(contents, "pam_oslogin_login.so") {
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
	if strings.Contains(contents, "pam_oslogin_login.so") {
		t.Errorf("OS Login PAM module wrongly included in pam.d/sshd when disabled.")
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
