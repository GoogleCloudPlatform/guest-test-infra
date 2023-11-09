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
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Creates the posix name from the given username.
func getPosix(user string) string {
	return strings.Join(strings.FieldsFunc(user, getPosixSplit), "_")
}

func getPosixSplit(r rune) bool {
	return r == '.' || r == '@' || r == '-'
}

// Gets the name of the instance running the test.
func getInstanceName(ctx context.Context) (string, error) {
	name, err := utils.GetMetadata(ctx, "instance", "name")
	if err != nil {
		return "", fmt.Errorf("failed to get instance name: %v", err)
	}
	return name, nil
}

// Gets the project and zone of the instance.
func getProjectZone(ctx context.Context) (string, string, error) {
	projectZone, err := utils.GetMetadata(ctx, "instance", "zone")
	if err != nil {
		return "", "", fmt.Errorf("failed to get instance zone: %v", err)
	}
	projectZoneSplit := strings.Split(string(projectZone), "/")
	project := projectZoneSplit[1]
	zone := projectZoneSplit[3]
	return project, zone, nil
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
		return fmt.Errorf("cannot read /etc/nsswitch.conf")
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "passwd:") && !strings.Contains(line, "oslogin") {
			return fmt.Errorf("OS Login not enabled in /etc/nsswitch.conf")
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
		return fmt.Errorf("AuthorizedKeysCommand not set up for OS Login")
	}

	if err = testSSHDPamConfig(ctx); err != nil {
		return fmt.Errorf("error checking pam config: %v", err)
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

		if !strings.Contains(string(data), "pam_oslogin_login.so") {
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

// Gets the given secret.
func getSecret(ctx context.Context, client *secretmanager.Client, secretName string) (string, error) {
	// Get project
	project, _, err := getProjectZone(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %v", err)
	}

	// Make request call to Secret Manager.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, secretName),
	}
	resp, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %v", err)
	}
	return string(resp.Payload.Data), nil
}
