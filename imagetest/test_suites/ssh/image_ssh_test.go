package ssh

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"testing"

	"cloud.google.com/go/storage"
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
		t.Fatal("failed to get real vm name")
	}
	keyURL, err := utils.GetMetadataAttribute("_ssh_key_url")
	if err != nil {
		t.Fatalf("Couldn't get key path from metadata")
	}
	client, session, err := createSession(user, fmt.Sprintf("%s:22", vmname), keyURL)
	if err != nil {
		t.Fatalf("user %s failed ssh to target host, %s, err %v", user, vmname, err)
	}
	if err := session.Run("hostname"); err != nil {
		t.Fatal("failed to run cmd hostname")
	}
	if err := client.Close(); err != nil {
		t.Log("failed to close client")
	}
}

func createSession(user, host, keyPath string) (*ssh.Client, *ssh.Session, error) {
	pembytes, err := downloadPrivateKey(keyPath)
	if err != nil {
		panic(err)
	}
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

func downloadPrivateKey(gcsPath string) ([]byte, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create cloud storage client: %v", err)
	}
	privateKey, err := utils.DownloadGCSObject(ctx, client, gcsPath)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
