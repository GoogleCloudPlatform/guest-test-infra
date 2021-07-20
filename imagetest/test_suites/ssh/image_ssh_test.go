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
	client, session, err := createSession(user, fmt.Sprintf("%s:22", vmname), pembytes)
	if err != nil {
		t.Fatalf("user %s failed ssh to target host, %s, err %v", user, vmname, err)
	}
	if err := session.Run("hostname"); err != nil {
		t.Fatalf("failed to run cmd hostname: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Logf("failed to close client: %v", err)
	}
}

func createSession(user, host string, pembytes []byte) (*ssh.Client, *ssh.Session, error) {
	// generate signer instance from plain key
	signer, err := ssh.ParsePrivateKey(pembytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing plain private key failed %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, nil, err
	}
	return client, session, nil
}
