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
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"github.com/xlzd/gotp"
	"golang.org/x/crypto/ssh"
)

// testUser encapsulates a test user for this test.
type testUser struct {
	// email is the secret for the email of the test user.
	email string

	// is2FA dictates whether this user is a user for 2FA tests.
	is2FA bool

	// isAdmin dictates whether this user has admin OSLogin priveleges.
	isAdmin bool

	// twoFAKey is the secret for the 2FA secret for the test user.
	twoFAKey string

	// sshKey is the private SSH Key for the test user.
	sshKey string
}

const (
	// normal users
	normalUser    = "normal-user"
	adminUser     = "admin-user"

	// SSH keys
	normalUserSSHKey  = "normal-user-ssh-key"
	adminUserSSHKey   = "admin-user-ssh-key"

	// Time to wait for agent check. The agent check consists of 2 metadata waits
	// and 1-2s of runtime. This allows for the agent check to finish, with extra
	// padding for safety.
	waitAgent = time.Second * 15

	// Waiting for 3 seconds or less causes issues with the steps of TestAgent
	// starting before the guest agent is able to properly react to the metadata changes.
	waitMetadata = time.Second * 4
)

// Changes the given metadata key to have the given value on the given instance. If the key does not exist,
// then this will create the key-value pair in the instance's metadata.
func changeMetadata(ctx context.Context, client *compute.InstancesClient, key, value string) error {
	vmname, err := utils.GetInstanceName(ctx)
	if err != nil {
		return fmt.Errorf("error getting vm name: %v", err)
	}

	// Get project and zone of instance.
	project, zone, err := utils.GetProjectZone(ctx)
	if err != nil {
		return err
	}

	// Get instance info.
	instancesGetReq := &computepb.GetInstanceRequest{
		Instance: vmname,
		Project:  project,
		Zone:     zone,
	}

	instance, err := client.Get(ctx, instancesGetReq)
	if err != nil {
		return fmt.Errorf("error getting instance info: %v", err)
	}
	metadata := instance.Metadata

	// Find the key in the metadata. If it doesn't exist, create a new metadata item.
	found := false
	for _, item := range metadata.Items {
		if *(item.Key) == key {
			item.Value = &value
			found = true
			break
		}
	}
	if !found {
		metadata.Items = append(metadata.Items, &computepb.Items{Key: &key, Value: &value})
	}

	// Update the metadata on the instance.
	setMetadataReq := &computepb.SetMetadataInstanceRequest{
		Instance:         vmname,
		MetadataResource: metadata,
		Project:          project,
		Zone:             zone,
	}
	_, err = client.SetMetadata(ctx, setMetadataReq)
	if err != nil {
		return fmt.Errorf("error setting metadata: %v", err)
	}
	return nil
}

func sessionOSLoginEnabled(client *ssh.Client) error {
	// We do not close the session as Run() implicitly closes the session after it's done running.
	// Otherwise we run into an EOF error.
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create ssh session: %v", err)
	}
	data, err := session.Output("cat /etc/nsswitch.conf")
	if err != nil {
		return fmt.Errorf("failed to read /etc/nsswitch.conf: %v", err)
	}

	if err = fileContainsLine(string(data), "passwd:", "oslogin"); err != nil {
		return fmt.Errorf("oslogin passwd entry not found in /etc/nsswitch.conf")
	}

	return nil
}

// Checks what's in the /var/google-sudoers.d directory.
func getSudoFile(client *ssh.Client, user string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create ssh session: %v", err)
	}

	output, err := session.Output(fmt.Sprintf("sudo cat /var/google-sudoers.d/%v", user))
	if err != nil {
		return "", fmt.Errorf("error getting user's /var/google-sudoers.d file: %v", err)
	}
	return string(output), nil
}

// TestAgent checks whether the guest agent responds correctly to switching
// oslogin on and off.
func TestAgent(t *testing.T) {
	// Create instances client
	ctx := utils.Context(t)
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		t.Fatalf("failed to create instances client: %v", err)
	}
	defer client.Close()

	// First check if OSLogin is on.
	if err := isOsLoginEnabled(ctx); err != nil {
		t.Fatalf("OSLogin disabled when it should be enabled: %v", err)
	}
	// Turn off OsLogin.
	if err := changeMetadata(ctx, client, "enable-oslogin", "false"); err != nil {
		t.Fatalf("Error changing metadata: %v", err)
	}
	// Give the API time to update.
	time.Sleep(waitMetadata)
	// Check if OSLogin is disabled.
	err = isOsLoginEnabled(ctx)
	if err == nil {
		t.Fatalf("OSLogin enabled when it should be disabled: %v", err)
	} else if strings.Contains(err.Error(), "cannot read") {
		t.Fatalf("%v", err)
	}

	// Turn OSLogin back on.
	if err = changeMetadata(ctx, client, "enable-oslogin", "true"); err != nil {
		t.Fatalf("Error changing metadata: %v", err)
	}
	time.Sleep(waitMetadata)

	// Check if OSLogin is back on.
	if err = isOsLoginEnabled(ctx); err != nil {
		t.Fatalf("OSLogin disabled when it should be enabled: %v", err)
	}
}

// Checks whether SSH-ing works correctly with OSLogin enabled.
// After successfully creating an SSH connection, check whether OSLogin is enabled on the host VM.
func TestSSH(t *testing.T) {
	// TODO: Come up with better way to ensure the target VMs finished their guest agent checks.
	// Since this is the first test to be run with the current setup, this is the only test method
	// that would need to wait for the TestAgent test to finish on the target VMs.
	time.Sleep(waitAgent)
	ctx := utils.Context(t)

	// Secret Manager Client.
	secretClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create secrets client: %v", err)
	}
	defer secretClient.Close()

	// Get user email.
	user, err := utils.AccessSecret(ctx, secretClient, normalUser)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	// Get important name resources.
	hostname, err := utils.GetRealVMName("default")
	if err != nil {
		t.Fatalf("failed to get real vm name: %v", err)
	}
	posix := getPosix(user)

	// Get SSH keys.
	privateSSHKey, err := utils.AccessSecret(ctx, secretClient, normalUserSSHKey)
	if err != nil {
		t.Fatalf("failed to get private key: %v", err)
	}

	// Create ssh client to target VM.
	client, err := utils.CreateClient(posix, fmt.Sprintf("%s:22", hostname), []byte(privateSSHKey))
	if err != nil {
		t.Fatalf("error creating ssh client: %v", err)
	}
	defer client.Close()

	if err = sessionOSLoginEnabled(client); err != nil {
		t.Fatalf("%v", err)
	}
}

// TestAdminSSH checks if an admin user can use sudo after successfully SSH-ing to an instance.
func TestAdminSSH(t *testing.T) {
	ctx := utils.Context(t)

	// Secret Manager Client.
	secretClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create secrets client: %v", err)
	}
	defer secretClient.Close()

	// Get user email.
	user, err := utils.AccessSecret(ctx, secretClient, adminUser)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	// Get important name resources.
	hostname, err := utils.GetRealVMName("default")
	if err != nil {
		t.Fatalf("failed to get real vm name: %v", err)
	}
	posix := getPosix(user)

	// Get SSH keys.
	privateSSHKey, err := utils.AccessSecret(ctx, secretClient, adminUserSSHKey)
	if err != nil {
		t.Fatalf("failed to get private key: %v", err)
	}

	// Create ssh client to target VM.
	client, err := utils.CreateClient(posix, fmt.Sprintf("%s:22", hostname), []byte(privateSSHKey))
	if err != nil {
		t.Fatalf("error creating ssh client: %v", err)
	}
	defer client.Close()

	if err = sessionOSLoginEnabled(client); err != nil {
		t.Fatalf("%v", err)
	}

	data, err := getSudoFile(client, posix)
	if err != nil {
		t.Fatalf("failed to get sudo file: %v", err)
	}
	if !strings.Contains(data, posix) || !strings.Contains(data, "ALL=(ALL)") || !strings.Contains(data, "NOPASSWD: ALL") {
		t.Fatalf("sudoers file does not contain user or necessary configurations")
	}
}

// Test2FASSH tests if users set up with 2FA can SSH to a VM using 2FA OSLogin.
func Test2FASSH(t *testing.T) {
	ctx := utils.Context(t)

	// Obtain the secrets for the 2FA user to use for this test.
	userSecret, err := utils.GetMetadata(ctx, "instance", "attributes", normal2FAUser)
	if err != nil {
		t.Fatalf("failed to get secret for user: %v", err)
	}
	sshKeySecret, err := utils.GetMetadata(ctx, "instance", "attributes", normal2FASSHKey)
	if err != nil {
		t.Fatalf("failed to get secret for user ssh key: %v", err)
	}
	twoFASecret, err := utils.GetMetadata(ctx, "instance", "attributes", normal2FAKey)
	if err != nil {
		t.Fatalf("failed to get secret for user 2FA secret: %v", err)
	}

	// Secret manager client.
	secretClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create secret manager client: %v", err)
	}
	defer secretClient.Close()

	user, err := utils.AccessSecret(ctx, secretClient, userSecret)
	if err != nil {
		t.Fatalf("failed to get user info: %v", err)
	}
	posix := getPosix(user)

	privateSSHKey, err := utils.AccessSecret(ctx, secretClient, sshKeySecret)
	if err != nil {
		t.Fatalf("failed to get user ssh key: %v", err)
	}

	// Manually set up SSH.
	vmname, err := utils.GetRealVMName("twofa")
	if err != nil {
		t.Fatalf("failed to get hostname: %v", err)
	}
	hostname := fmt.Sprintf("%s:22", vmname)
	signer, err := ssh.ParsePrivateKey([]byte(privateSSHKey))
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	// This function handles the 2FA challenges.
	// For this test, we use the OTP from authenticator method, after which we can input a OTP.
	// The reason we go with this route is because there are libraries that allow us to
	// use a 2FA secret to generate the correct OTP. The other method, which depends on text
	// messages or phone calls, would be much harder to simulate.
	cb := func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		var answers []string

		if len(questions) == 0 {
			return answers, nil
		}

		answers = make([]string, 1)
		firstQuestionRegex, err := regexp.Compile(".*Enter.*number.*")
		if err != nil {
			return nil, err
		}
		if firstQuestionRegex.MatchString(questions[0]) {
			// 1 is the response for Authenticator OTP
			answers[0] = "1"
			return answers, nil
		}

		// If not the first question, input code for two-factor.
		s, err := utils.AccessSecret(ctx, secretClient, twoFASecret)
		if err != nil {
			return nil, err
		}
		secret := strings.ToUpper(s)

		if !gotp.IsSecretValid(secret) {
			// Avoid panic that doesn't return a useful error message for test runner
			return nil, fmt.Errorf("invalid secret")
		}
		code := gotp.NewDefaultTOTP(secret).Now()
		answers[0] = code
		return answers, nil
	}

	sshConfig := &ssh.ClientConfig{
		User:            posix,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer), ssh.KeyboardInteractive(cb)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", hostname, sshConfig)
	if err != nil {
		t.Fatalf("failed to create ssh client: %v", err)
	}
	defer client.Close()

	if err = sessionOSLoginEnabled(client); err != nil {
		t.Fatalf("%v", err)
	}
}

// Test2FAAdminSSH tests whether 2FA authentication with OSLogin admin permissions
// correctly gives admin users sudo permissions.
func Test2FAAdminSSH(t *testing.T) {
	ctx := utils.Context(t)

	// Obtain the secrets for the 2FA user to use for this test.
	userSecret, err := utils.GetMetadata(ctx, "instance", "attributes", admin2FAUser)
	if err != nil {
		t.Fatalf("failed to get secret for user: %v", err)
	}
	sshKeySecret, err := utils.GetMetadata(ctx, "instance", "attributes", admin2FASSHKey)
	if err != nil {
		t.Fatalf("failed to get secret for user ssh key: %v", err)
	}
	twoFASecret, err := utils.GetMetadata(ctx, "instance", "attributes", admin2FAKey)
	if err != nil {
		t.Fatalf("failed to get secret for user 2FA secret: %v", err)
	}

	// Secret manager client.
	secretClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create secret manager client: %v", err)
	}
	defer secretClient.Close()

	user, err := utils.AccessSecret(ctx, secretClient, userSecret)
	if err != nil {
		t.Fatalf("failed to get user info: %v", err)
	}
	posix := getPosix(user)

	privateSSHKey, err := utils.AccessSecret(ctx, secretClient, sshKeySecret)
	if err != nil {
		t.Fatalf("failed to get user key: %v", err)
	}

	// Manually set up SSH.
	vmname, err := utils.GetRealVMName("twofa")
	if err != nil {
		t.Fatalf("failed to get hostname: %v", err)
	}
	hostname := fmt.Sprintf("%s:22", vmname)
	signer, err := ssh.ParsePrivateKey([]byte(privateSSHKey))
	if err != nil {
		t.Fatalf("failed to parse private key: %v", err)
	}

	// This function handles the 2FA challenges.
	// For this test, we use the OTP from authenticator method, after which we can input a OTP.
	// The reason we go with this route is because there are libraries that allow us to
	// use a 2FA secret to generate the correct OTP. The other method, which depends on text
	// messages or phone calls, would be much harder to simulate.
	cb := func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		var answers []string

		if len(questions) == 0 {
			return answers, nil
		}

		answers = make([]string, 1)
		firstQuestionRegex, err := regexp.Compile(".*Enter.*number.*")
		if err != nil {
			return nil, err
		}
		if firstQuestionRegex.MatchString(questions[0]) {
			// 1 is the response for Authenticator OTP
			answers[0] = "1"
			return answers, nil
		}

		// If not the first question, generate and input the OTP.
		s, err := utils.AccessSecret(ctx, secretClient, twoFASecret)
		if err != nil {
			return nil, err
		}
		secret := strings.ToUpper(s)

		if !gotp.IsSecretValid(secret) {
			// Avoid panic that doesn't return a useful error message for test runner
			return nil, fmt.Errorf("invalid secret")
		}
		code := gotp.NewDefaultTOTP(secret).Now()
		answers[0] = code
		return answers, nil
	}

	sshConfig := &ssh.ClientConfig{
		User:            posix,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer), ssh.KeyboardInteractive(cb)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", hostname, sshConfig)
	if err != nil {
		t.Fatalf("failed to create ssh client: %v", err)
	}
	defer client.Close()

	// Check OSLogin enabled on the server instance.
	if err = sessionOSLoginEnabled(client); err != nil {
		t.Fatalf("%v", err)
	}

	// Check contents of the sudo file.
	data, err := getSudoFile(client, posix)
	if err != nil {
		t.Fatalf("failed to get sudo file: %v", err)
	}
	if !strings.Contains(data, posix) || !strings.Contains(data, "ALL=(ALL)") || !strings.Contains(data, "NOPASSWD: ALL") {
		t.Fatalf("sudoers file does not contain user or necessary configurations")
	}
}
