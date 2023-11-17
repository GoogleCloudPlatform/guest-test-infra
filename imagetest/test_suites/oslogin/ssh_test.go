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

const (
	// users
	normalUser    = "normal-user"
	adminUser     = "admin-user"
	normal2FAUser = "normal-2fa-user"
	admin2FAUser  = "admin-2fa-user"

	// 2fa keys
	normal2FAKey = "normal-2fa-key"
	admin2FAKey  = "admin-2fa-key"

	// SSH keys
	normalUserSSHKey = "normal-user-ssh-key"
	adminUserSSHKey  = "admin-user-ssh-key"
	normal2FASSHKey  = "normal-2fa-ssh-key"
	admin2FASSHKey   = "admin-2fa-ssh-key"

	// Time to wait for agent check
	waitAgent = time.Second * 15
)

// Changes the given metadata key to have the given value on the given instance.. If the key does not exist,
// then this will create the key-value pair in the instance's metadata.
func changeMetadata(ctx context.Context, client *compute.InstancesClient, key, value string) error {
	vmname, err := getInstanceName(ctx)
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
	time.Sleep(time.Second * 4)
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
	time.Sleep(time.Second * 4)

	// Check if OSLogin is back on.
	if err = isOsLoginEnabled(ctx); err != nil {
		t.Fatalf("OSLogin disabled when it should be enabled: %v", err)
	}
}

// Checks whether SSH-ing works correctly with OSLogin enabled.
// After successfully creating an SSH connection, check whether OSLogin is enabled on the host VM.
func TestSSH(t *testing.T) {
	// TODO: Come up with better way to ensure the target VMs finished their guest agent checks.
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

// Checks if an admin user can use sudo after successfully SSH-ing to an instance.
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

func Test2FASSH(t *testing.T) {
	ctx := utils.Context(t)

	// Secret manager client.
	secretClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create secret manager client: %v", err)
	}
	defer secretClient.Close()

	user, err := utils.AccessSecret(ctx, secretClient, normal2FAUser)
	if err != nil {
		t.Fatalf("failed to get user info: %v", err)
	}
	posix := getPosix(user)

	privateSSHKey, err := utils.AccessSecret(ctx, secretClient, normal2FASSHKey)
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

	cb := func(name, instruction string, questions []string, echos []bool) (answers []string, err error) {
		if len(questions) == 0 {
			return
		}

		answers = make([]string, 1)
		firstQuestionRegex, err := regexp.Compile(".*Enter.*number.*")
		if err != nil {
			return
		}
		if firstQuestionRegex.MatchString(questions[0]) {
			// 1 is the response for Authenticator OTP
			answers[0] = "1"
			return
		}

		// If not the first question, input code for two-factor.
		ctx := context.Background()
		secretClient, err := secretmanager.NewClient(ctx)
		if err != nil {
			return
		}
		defer secretClient.Close()

		s, err := utils.AccessSecret(ctx, secretClient, normal2FAKey)
		if err != nil {
			return
		}
		secret := strings.ToUpper(s)

		if !gotp.IsSecretValid(secret) {
			// Avoid panic that doesn't return a useful error message for test runner
			err = fmt.Errorf("invalid secret")
			return
		}
		code := gotp.NewDefaultTOTP(secret).Now()
		answers[0] = code
		return
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

func Test2FAAdminSSH(t *testing.T) {
	ctx := utils.Context(t)

	// Secret manager client.
	secretClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		t.Fatalf("failed to create secret manager client: %v", err)
	}
	defer secretClient.Close()

	user, err := utils.AccessSecret(ctx, secretClient, admin2FAUser)
	if err != nil {
		t.Fatalf("failed to get user info: %v", err)
	}
	posix := getPosix(user)

	privateSSHKey, err := utils.AccessSecret(ctx, secretClient, admin2FASSHKey)
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

	// Callback function for SSH keyboard-interactive challenges.
	cb := func(name, instruction string, questions []string, echos []bool) (answers []string, err error) {
		if len(questions) == 0 {
			return
		}

		answers = make([]string, 1)
		firstQuestionRegex, err := regexp.Compile(".*Enter.*number.*")
		if err != nil {
			return
		}
		if firstQuestionRegex.MatchString(questions[0]) {
			// 1 is the response for Authenticator OTP
			answers[0] = "1"
			return
		}

		// If not the first question, input code for two-factor.
		ctx := context.Background()
		secretClient, err := secretmanager.NewClient(ctx)
		if err != nil {
			return
		}
		defer secretClient.Close()

		s, err := utils.AccessSecret(ctx, secretClient, admin2FAKey)
		if err != nil {
			return
		}
		secret := strings.ToUpper(s)

		if !gotp.IsSecretValid(secret) {
			// Avoid panic that doesn't return a useful error message for test runner
			err = fmt.Errorf("invalid secret")
			return
		}
		code := gotp.NewDefaultTOTP(secret).Now()
		answers[0] = code
		return
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
