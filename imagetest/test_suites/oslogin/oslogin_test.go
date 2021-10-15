//go:build cit
// +build cit

package oslogin

import (
	"bufio"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestOsLoginEnabled(t *testing.T) {
	// Check OS Login enabled in /etc/nsswitch.conf
	file, err := os.Open("/etc/nsswitch.conf")
	if err != nil {
		t.Fatalf("cannot read /etc/nsswitch.conf")
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "passwd:") {
			if !strings.Contains(scanner.Text(), "oslogin") {
				t.Errorf("OS Login not enabled in /etc/nsswitch.conf.")
			}
		}

	}
	file.Close()

	// Check AuthorizedKeys Command
	file, err = os.Open("/etc/ssh/sshd_config")
	if err != nil {
		t.Fatalf("cannot read /etc/ssh/sshd_config")
	}
	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "AuthorizedKeysCommand ") {
			if !strings.Contains(scanner.Text(), "/usr/bin/google_authorized_keys") {
				t.Errorf("AuthorizedKeysCommand not set up for OS Login.")
			}
		}

	}
	file.Close()

	// Check Pam Modules
	data, err := ioutil.ReadFile("/etc/pam.d/sshd")
	if err != nil {
		t.Fatalf("cannot read /etc/pam.d/sshd")
	}
	contents := string(data)
	if !strings.Contains(contents, "pam_oslogin_login.so") {
		t.Errorf("OS Login module pam_oslogin_login.so not set up.")
	}
	if !strings.Contains(contents, "pam_oslogin_admin.so") {
		t.Errorf("OS Login module pam_oslogin_admin.so not set up.")
	}
}

func TestOsLoginDisabled(t *testing.T) {
	// Check OS Login not enabled in /etc/nsswitch.conf
	file, err := os.Open("/etc/nsswitch.conf")
	if err != nil {
		t.Fatalf("cannot read /etc/nsswitch.conf")
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "passwd:") {
			if strings.Contains(scanner.Text(), "oslogin") {
				t.Errorf("OS Login enabled in /etc/nsswitch.conf.")
			}
		}

	}
	file.Close()

	// Check AuthorizedKeys Command
	file, err = os.Open("/etc/ssh/sshd_config")
	if err != nil {
		t.Fatalf("cannot read /etc/ssh/sshd_config")
	}
	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), "AuthorizedKeysCommand ") {
			if strings.Contains(scanner.Text(), "/usr/bin/google_authorized_keys") {
				t.Errorf("AuthorizedKeysCommand is set up for OS Login.")
			}
		}

	}
	file.Close()

	// Check Pam Modules
	data, err := ioutil.ReadFile("/etc/pam.d/sshd")
	if err != nil {
		t.Fatalf("cannot read /etc/pam.d/sshd")
	}
	contents := string(data)
	if strings.Contains(contents, "pam_oslogin_login.so") {
		t.Errorf("OS Login module pam_oslogin_login.so is set up.")
	}
	if strings.Contains(contents, "pam_oslogin_admin.so") {
		t.Errorf("OS Login module pam_oslogin_admin.so is set up.")
	}
}
