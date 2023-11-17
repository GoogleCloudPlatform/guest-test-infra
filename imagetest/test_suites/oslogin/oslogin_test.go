//go:build cit
// +build cit

// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oslogin

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestOsLoginEnabled(t *testing.T) {
	if err := isOsLoginEnabled(utils.Context(t)); err != nil {
		t.Fatalf(err.Error())
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

	if err = testSSHDPamConfig(utils.Context(t)); err != nil {
		t.Fatalf("error checking pam config: %v", err)
	}
}

func TestGetentPasswdOsloginUser(t *testing.T) {
	testUsername, _, testUserEntry, err := getTestUserEntry(utils.Context(t))
	if err != nil {
		t.Fatalf("failed to get test user entry: %v", err)
	}

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
	_, _, testUserEntry, err := getTestUserEntry(utils.Context(t))
	if err != nil {
		t.Fatalf("failed to get test user entry: %v", err)
	}

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
	_, testUUID, testUserEntry, err := getTestUserEntry(utils.Context(t))
	if err != nil {
		t.Fatalf("failed to get test user entry: %v", err)
	}

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
