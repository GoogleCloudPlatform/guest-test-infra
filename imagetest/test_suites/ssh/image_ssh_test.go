// +build cit

package ssh

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"golang.org/x/crypto/ssh"
)

func TestEmptyTest(t *testing.T) {
	_, err := utils.GetMetadataAttribute("ssh-keys")
	if err != nil {
		t.Fatalf("couldn't get ssh public key from metadata")
	}
	t.Logf("ssh target boot succesfully at %d", time.Now().UnixNano())
}

func TestSSH(t *testing.T) {
	vmname, err := utils.GetRealVMName("vm2")
	if err != nil {
		t.Fatalf("failed to get real vm name: %v", err)
	}
	pembytes, err := utils.DownloadPrivateKey(user)
	if err != nil {
		t.Fatalf("failed to download private key: %v", err)
	}
	client, err := utils.CreateClient(user, fmt.Sprintf("%s:22", vmname), pembytes)
	time.Sleep(60 * time.Second)
	t.Logf("connect to remote host at %d", time.Now().UnixNano())
	client, err := createClient(user, fmt.Sprintf("%s:22", vmname), pembytes)
	if err != nil {
		t.Fatalf("user %s failed ssh to target host, %s, err %v", user, vmname, err)
	}
	if err := checkLocalUser(client, user); err != nil {
		t.Fatalf("failed to check local user: %v", err)
	}

	if err := checkSudoGroup(client, user); err != nil {
		t.Fatalf("failed to check sudo group: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Logf("failed to close client: %v", err)
	}
}

// checkLocalUser test that the user account exists in /etc/passwd
func checkLocalUser(client *ssh.Client, user string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	grepPasswdCmd := fmt.Sprintf("grep %s: /etc/passwd", user)
	if err := session.Run(grepPasswdCmd); err != nil {
		return err
	}
	return nil
}

// checkSudoGroup test that the user account exists in sudo group
func checkSudoGroup(client *ssh.Client, user string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	grepGroupCmd := fmt.Sprintf("grep 'google-sudoers:.*%s' /etc/group", user)
	if err := session.Run(grepGroupCmd); err != nil {
		return err
	}
	return nil
}

func TestHostKeysAreUnique(t *testing.T) {
	vmname, err := utils.GetRealVMName("vm2")
	if err != nil {
		t.Fatalf("failed to get real vm name: %v", err)
	}
	pembytes, err := utils.DownloadPrivateKey(user)
	if err != nil {
		t.Fatalf("failed to download private key: %v", err)
	}
	time.Sleep(60 * time.Second)
	t.Logf("connect to remote host at %d", time.Now().UnixNano())
	client, err := createClient(user, fmt.Sprintf("%s:22", vmname), pembytes)
	if err != nil {
		t.Fatalf("user %s failed ssh to target host, %s, err %v", user, vmname, err)
	}
	remoteDiskEntries, err := getRemoteHostKey(client)
	if err != nil {
		t.Fatalf("failed to get host key from remote, err: %v", err)
	}

	localDiskEntries, err := utils.GetHostKeysFromDisk()
	if err != nil {
		t.Fatalf("failed to get host key from disk %v", err)
	}
	for keyType, localValue := range localDiskEntries {
		remoteValue, found := remoteDiskEntries[keyType]
		if !found {
			t.Fatalf("ssh key %s not found on remote disk entries", keyType)
		}
		if localValue == remoteValue {
			t.Fatal("host key value is not unique")
		}
	}
}

func getRemoteHostKey(client *ssh.Client) (map[string]string, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	bytes, err := session.Output("cat /etc/ssh/ssh_host_*_key.pub")
	if err != nil {
		return nil, err
	}
	return utils.ParseHostKey(bytes)
}

// TestMatchingKeysInGuestAttributes validate that host keys in guest attributes match those on disk.
func TestMatchingKeysInGuestAttributes(t *testing.T) {
	diskEntries, err := utils.GetHostKeysFromDisk()
	if err != nil {
		t.Fatalf("failed to get host key from disk %v", err)
	}

	hostkeys, err := utils.GetMetadataGuestAttribute("hostkeys/")
	if err != nil {
		t.Fatal(err)

	}
	// validate that the guest agent copies the host keys from disk to the metadata.
	// https://github.com/GoogleCloudPlatform/guest-agent/blob/main/google_guest_agent/instance_setup.go
	for _, keyType := range strings.Split(hostkeys, "\n") {
		if keyType == "" {
			continue
		}
		keyValue, err := utils.GetMetadataGuestAttribute("hostkeys/" + keyType)
		if err != nil {
			t.Fatal(err)
		}
		valueFromDisk, found := diskEntries[keyType]
		if !found {
			t.Fatalf("failed finding key %s from disk", keyType)
		}
		if valueFromDisk != strings.TrimSpace(keyValue) {
			t.Fatalf("host keys %s %s in guest attributes match those on disk %s", keyType, keyValue, valueFromDisk)
		}
	}
}
