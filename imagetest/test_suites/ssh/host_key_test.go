// +build cit

package ssh

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"golang.org/x/crypto/ssh"
)

const (
	markerFile     = "/boot-marker"
)

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
			t.Fatalf("host keys %s %s in guest attributes not match those on disk %s", keyType, keyValue, valueFromDisk)
		}
	}
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
	client, err := utils.CreateClient(user, fmt.Sprintf("%s:22", vmname), pembytes)
	if err != nil {
		t.Fatalf("user %s failed ssh to target host, %s, err %v", user, vmname, err)
	}
	remoteDiskEntries, err := getRemoteHostKeys(client)
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

func getRemoteHostKeys(client *ssh.Client) (map[string]string, error) {
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

func TestHostKeysNotOverrideAfterReboot(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		hostKeys, err := utils.GetHostKeysFromDisk()
		if err != nil {
			t.Fatalf("failed to get host key from disk %v", err)
		}
		file, err := os.Create("/hostkeys")
		if err != nil {
			t.Fatalf("failed creating hostkeys file: %v", err)
		}
		var hostkeysStr []string
		for key, value :=range hostKeys {
			hostkeysStr = append(hostkeysStr, fmt.Sprintf("%s %s", key, value))
		}
		if _, err:= file.WriteString(strings.Join(hostkeysStr, "\n")); err!=nil{
			t.Fatalf("failed writting data to file %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	hostKeys, err := utils.GetHostKeysFromDisk()
	if err != nil {
		t.Fatalf("failed to get host key from disk %v", err)
	}

	data, err := ioutil.ReadFile("/hostkeys")
	if err != nil {
		t.Fatalf("failed reading hostkeys file: %v", err)
	}
	splits := strings.Split(string(data), "\n")
	for _, line := range splits {
		keyType := strings.Split(line, " ")[0]
		keyValue := strings.Split(line, " ")[1]
		afterReboot, found := hostKeys[keyType]
		if !found{
			t.Fatalf("host keys %s are not found after reboot", keyType)
		}
		if afterReboot != keyValue {
			t.Fatalf("host keys on first boot %s %s change after reboot", keyType, keyValue)
		}
	}
}