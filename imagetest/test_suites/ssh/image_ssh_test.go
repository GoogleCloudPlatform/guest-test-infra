package ssh

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"golang.org/x/crypto/ssh"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

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
