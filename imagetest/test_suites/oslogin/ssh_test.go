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
	"strings"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
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

	// keys
	normalUserSshKey = "normal-user-ssh-key"
	adminUserSshKey  = "admin-user-ssh-key"
	normal2FASshKey  = "normal-2fa-ssh-key"
	admin2FASshKey   = "admin-2fa-ssh-key"
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
func getSudoFile(client *ssh.Client, user, sudo string) (string, error) {
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
	time.Sleep(time.Second * 5)
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
	time.Sleep(time.Second * 5)

	// Check if OSLogin is back on.
	if err = isOsLoginEnabled(ctx); err != nil {
		t.Fatalf("OSLogin disabled when it should be enabled: %v", err)
	}
}

// Checks whether SSH-ing works correctly with OSLogin enabled.
// After successfully creating an SSH connection, check whether OSLogin is enabled on the host VM.
func TestSSH(t *testing.T) {
	// TODO: Come up with better way to ensure the target VMs finished their guest agent checks.
	time.Sleep(20 * time.Second)
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

	// Get Ssh keys.
	privateSshKey, err := utils.AccessSecret(ctx, secretClient, normalUserSshKey)
	if err != nil {
		t.Fatalf("failed to get private key: %v", err)
	}

	// Create ssh client to target VM.
	client, err := utils.CreateClient(posix, fmt.Sprintf("%s:22", hostname), []byte(privateSshKey))
	if err != nil {
		t.Fatalf("error creating ssh client: %v", err)
	}
	defer client.Close()

	if err = sessionOSLoginEnabled(client); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestAdminSSH(t *testing.T) {
	time.Sleep(20 * time.Second)
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

	// Get Ssh keys.
	privateSshKey, err := utils.AccessSecret(ctx, secretClient, adminUserSshKey)
	if err != nil {
		t.Fatalf("failed to get private key: %v", err)
	}

	// Create ssh client to target VM.
	client, err := utils.CreateClient(posix, fmt.Sprintf("%s:22", hostname), []byte(privateSshKey))
	if err != nil {
		t.Fatalf("error creating ssh client: %v", err)
	}
	defer client.Close()

	if err = sessionOSLoginEnabled(client); err != nil {
		t.Fatalf("%v", err)
	}

	sudo, err := utils.AccessSecret(ctx, secretClient, adminUserSudo)
	if err != nil {
		t.Fatalf("failed to get sudo: %v", err)
	}
	data, err := getSudoFile(client, posix, sudo)
	if err != nil {
		t.Fatalf("failed to get sudo file: %v", err)
	}
	if !strings.Contains(data, posix) && !strings.Contains(data, "ALL=(ALL)") || !strings.Contains(data, "NOPASSWD: ALL") {
		t.Fatalf("sudoers file does not contain user or necessary configurations")
	}
}
