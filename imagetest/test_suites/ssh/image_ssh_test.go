// +build cit

package ssh

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"golang.org/x/crypto/ssh"
)

func TestEmptyTest(t *testing.T) {
	t.Logf("SSH target boot succesfully")
	_, err := utils.GetMetadataAttribute("ssh-keys")
	if err != nil {
		t.Fatalf("couldn't get ssh public key from metadata")
	}
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

func createClient(user, host string, pembytes []byte) (*ssh.Client, error) {
	// generate signer instance from plain key
	signer, err := ssh.ParsePrivateKey(pembytes)
	if err != nil {
		return nil, fmt.Errorf("parsing plain private key failed %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
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
	client, err := createClient(user, fmt.Sprintf("%s:22", vmname), pembytes)
	if err != nil {
		t.Fatalf("user %s failed ssh to target host, %s, err %v", user, vmname, err)
	}
	bytes, err := getRemoteHostKey(client)
	if err != nil {
		t.Fatalf("failed to get host key from remote err %v", err)
	}
	remoteDiskEntries := utils.ParseHostKey(bytes)

	localDiskEntries, err := utils.GetHostKeysFromDisk()
	if err != nil {
		t.Fatalf("failed to get host key from disk %v", err)
	}
	for keyType, keyValue := range localDiskEntries {
		value, found := remoteDiskEntries[keyType];
		if !found {
			t.Fatalf("ssh key %s not found on remote disk entries", keyType)
		}
		if value == keyValue {
			t.Fatal("host key value is not unique")
		}
	}
}

func getRemoteHostKey(client *ssh.Client) ([]byte, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	bytes, err := session.Output("cat /etc/ssh/ssh_host_*_key.pub")
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
