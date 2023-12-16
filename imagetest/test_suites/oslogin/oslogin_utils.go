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
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	oslogin "cloud.google.com/go/oslogin/apiv1"
	osloginpb "cloud.google.com/go/oslogin/apiv1/osloginpb"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// From the contents of a file, check if a line contains all the provided elements.
func fileContainsLine(data string, elem ...string) error {
	var found bool
	for _, line := range strings.Split(string(data), "\n") {
		found = true
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		for _, s := range elem {
			found = found && strings.Contains(line, s)
		}
		if found {
			return nil
		}
	}
	return fmt.Errorf("elements not found")
}

// Creates the posix name from the given username.
func getPosix(user string) string {
	return strings.Join(strings.FieldsFunc(user, getPosixSplit), "_")
}

func getPosixSplit(r rune) bool {
	return r == '.' || r == '@' || r == '-'
}

// Gets the service account currently operating on the instance.
func getServiceAccount(ctx context.Context) (string, error) {
	serviceAccount, err := utils.GetMetadata(ctx, "instance", "service-accounts", "default", "email")
	if err != nil {
		return "", fmt.Errorf("failed to get service account: %v", err)
	}
	return serviceAccount, nil
}

// Gets the test user entry for getent tests. Returns the username, uuid, and entry.
func getTestUserEntry(ctx context.Context) (string, string, string, error) {
	account, err := getServiceAccount(ctx)
	if err != nil {
		return "", "", "", err
	}

	// Create OSLogin client
	client, err := oslogin.NewClient(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create client: %v", err)
	}

	// Get the LoginProfile for the service account.
	req := &osloginpb.GetLoginProfileRequest{
		Name: fmt.Sprintf("users/%s", account),
	}

	resp, err := client.GetLoginProfile(ctx, req)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get login profile: %v", err)
	}
	posixAccount := resp.PosixAccounts[0]

	// Get necessary information.
	uuid := posixAccount.GetUid()
	username := posixAccount.GetUsername()
	entry := fmt.Sprintf("%s:*:%d:%d::/home/%s:", username, uuid, uuid, username)
	return username, strconv.FormatInt(uuid, 10), entry, nil
}

// Checks if OSLogin is enabled. Returns an error if it is not, or there is trouble
// reading a file.
func isOsLoginEnabled(ctx context.Context) error {
	data, err := os.ReadFile("/etc/nsswitch.conf")
	if err != nil {
		return fmt.Errorf("cannot read /etc/nsswitch.conf: %v", err)
	}
	if err = fileContainsLine(string(data), "passwd:", "oslogin"); err != nil {
		return fmt.Errorf("oslogin passwd entry not found in /etc/nsswitch.conf")
	}

	// Check AuthorizedKeys Command
	data, err = os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return fmt.Errorf("cannot read /etc/ssh/sshd_config: %v", err)
	}
	if err = fileContainsLine(string(data), "AuthorizedKeysCommand", "/usr/bin/google_authorized_keys"); err != nil {
		if err = fileContainsLine(string(data), "AuthorizedKeysCommand", "/usr/bin/google_authorized_keys_sk"); err != nil {
			return fmt.Errorf("AuthorizedKeysCommand not set up for OSLogin: %v", err)
		}
	}

	if err = testSSHDPamConfig(ctx); err != nil {
		return err
	}
	return nil
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
		dataString := string(data)

		if err = fileContainsLine(dataString, "auth", "[success=done perm_denied=die default=ignore]", "pam_oslogin_login.so"); err != nil {
			return fmt.Errorf("OS Login PAM module missing from pam.d/sshd")
		}
	}
	return nil
}

func isTwoFactorAuthEnabled(ctx context.Context) (bool, error) {
	var (
		instanceFlag, projectFlag bool
		err                       error
	)

	elem := []string{"attributes", "enable-oslogin-2fa"}

	instanceFlag, err = getTwoFactorAuthMetadata(ctx, "instance", elem...)
	if err != nil && !errors.Is(err, utils.ErrMDSEntryNotFound) {
		return false, err
	}
	projectFlag, err = getTwoFactorAuthMetadata(ctx, "project", elem...)
	if err != nil && !errors.Is(err, utils.ErrMDSEntryNotFound) {
		return false, err
	}
	return instanceFlag || projectFlag, nil
}

func getTwoFactorAuthMetadata(ctx context.Context, root string, elem ...string) (bool, error) {
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
