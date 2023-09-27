//go:build cit
// +build cit

package oslogin

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const testUsername = "sa_105020877179577573373"
const testUUID = "3651018652"

const testGroupName = "demo"
const testRealGroup = "realdemo"
const testVirtGroup = "virtdemo"
const testGID = "123452"

var testUserEntry = fmt.Sprintf("%s:*:%s:%s::/home/%s:", testUsername, testUUID, testUUID, testUsername)

func TestOsLoginEnabled(t *testing.T) {
	// Check OS Login enabled in /etc/nsswitch.conf
	data, err := os.ReadFile("/etc/nsswitch.conf")
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
	data, err = os.ReadFile("/etc/ssh/sshd_config")
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
	data, err = os.ReadFile("/etc/pam.d/sshd")
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

	// Check Pam Modules
	data, err = os.ReadFile("/etc/pam.d/sshd")
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

func TestGetentGroupAllGroups(t *testing.T) {
	_, err := os.ReadFile("/etc/oslogin_group.cache")
	if err != nil {
		t.Fatalf("Error reading oslogin group cache")
	}
	out, err := exec.Command("getent", "group").Output()
	if err != nil {
		t.Fatalf("getent command failed %v", err)
	}
	if !strings.Contains(string(out), "root:x:0") {
		t.Fatalf("getent group output does not contain root")
	}
	if !strings.Contains(string(out), testGroupName) {
		t.Fatalf("getent group output does not contain test group")
	}
}

func TestGetentGroupInvalidGroup(t *testing.T) {
	_, err := exec.Command("getent", "group", "__invalid_group__").Output()
	if err == nil || err.Error() != "exit status 2" {
		t.Fatalf("getent group did not give error on invalid group")
	}
}

func TestGetentGroupLocalGroup(t *testing.T) {
	out, err := exec.Command("getent", "group", "root").Output()
	if err != nil {
		t.Fatalf("getent group failed %v", err)
	}
	if !strings.Contains(string(out), "root:x") {
		t.Fatalf("getent group does not contain root")
	}
}

func TestGetentGroupOsLoginGid(t *testing.T) {
	out, err := exec.Command("getent", "group", testGID).Output()
	if err != nil {
		t.Fatalf("getent group failed %v", err)
	}
	if !strings.Contains(string(out), testGroupName) {
		t.Fatalf("getent group does not contain test group")
	}
}

func TestGetentGroupOsLoginUIDGroup(t *testing.T) {
	out, err := exec.Command("getent", "group", testUUID).Output()
	if err != nil {
		t.Fatalf("getent group failed %v", err)
	}
	if !strings.Contains(string(out), testUsername) {
		t.Fatalf("getent group does not contain test username")
	}
}

func TestGetentGroupOsLoginGroupName(t *testing.T) {
	out, err := exec.Command("getent", "group", testGroupName).Output()
	if err != nil {
		t.Fatalf("getent group failed %v", err)
	}
	if !strings.Contains(string(out), testGroupName) {
		t.Fatalf("getent group does not contain test group")
	}
}

func testGetentGroupSelfGroup(groupName string) error {
	out, err := exec.Command("getent", "passwd", groupName).Output()
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), groupName) {
		return fmt.Errorf("getent does not contain test group")
	}

	gid := strings.Split(testUsername, ":")[3]
	out, err = exec.Command("getent", "group", gid).Output()
	if err != nil {
		return fmt.Errorf("getend group failed %v", err)
	}
	if !strings.Contains(string(out), groupName) {
		return fmt.Errorf("getent group does not contain test group")
	}
	return nil
}

func TestGetentGroupRealSelfGroup(t *testing.T) {
	if err := testGetentGroupSelfGroup(testRealGroup); err != nil {
		t.Fatalf("Error: %v", err)
	}
}

func TestGetentGroupVirtSelfGroup(t *testing.T) {
	if err := testGetentGroupSelfGroup(testVirtGroup); err != nil {
		t.Fatalf("Error: %v", err)
	}
}
