package image_validation

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/test_utils"
)

func TestHostname(t *testing.T) {
	metadataHostname, err := test_utils.GetMetadata("hostname")
	if err != nil {
		t.Fatalf(" still couldn't determine metadata hostname")
	}

	// 'hostname' in metadata is fully qualified domain name.
	shortname := strings.Split(metadataHostname, ".")[0]

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("couldn't determine local hostname")
	}

	if hostname != shortname {
		t.Errorf("hostname does not match metadata. Expected: %q got: %q", shortname, hostname)
	}

	// If hostname is FQDN then lots of tools (e.g. ssh-keygen) have issues
	if strings.Contains(hostname, ".") {
		t.Errorf("hostname contains '.'")
	}
}

// TestCustomHostname tests the 'fully qualified domain name', using the logic in the `hostname` utility.
func TestFQDN(t *testing.T) {
	metadataHostname, err := test_utils.GetMetadata("hostname")
	if err != nil {
		t.Fatalf("couldn't determine metadata hostname")
	}

	// This command is not safe on multi-NIC VMs. See HOSTNAME(1), section 'THE FQDN'.
	cmd := exec.Command("/bin/hostname", "-A")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("hostname command failed")
	}
	hostname := strings.TrimRight(string(out), " \n")

	if hostname != metadataHostname {
		t.Errorf("hostname does not match metadata. Expected: %q got: %q", metadataHostname, hostname)
	}
}

func TestArePackagesLegalToUse(t *testing.T) {
	// This will be a bit longer to translate ;)
	return
}

func md5Sum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("couldn't open file: %v", err)
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type sshKeyHash struct {
	file os.FileInfo
	hash string
}

// TestHostKeysGeneratedOnces checks that the guest agent only generates host keys one time.
func TestHostKeysGeneratedOnce(t *testing.T) {
	sshDir := "/etc/ssh/"
	sshfiles, err := ioutil.ReadDir(sshDir)
	if err != nil {
		t.Fatalf("Couldn't read files from ssh dir")
	}

	var hashes []sshKeyHash
	for _, file := range sshfiles {
		if !strings.HasSuffix(file.Name(), "_key.pub") {
			continue
		}
		hash, err := md5Sum(sshDir + file.Name())
		if err != nil {
			t.Fatalf("Couldn't hash file: %v", err)
		}
		hashes = append(hashes, sshKeyHash{file, hash})
	}

	image, err := test_utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata")
	}

	var restart string
	switch {
	case strings.Contains(image, "rhel-6"), strings.Contains(image, "centos-6"):
		restart = "initctl"
	default:
		restart = "systemctl"
	}

	cmd := exec.Command(restart, "restart", "google-guest-agent")
	err = cmd.Run()
	if err != nil {
		t.Errorf("Failed to restart guest agent: %v", err)
	}

	sshfiles, err = ioutil.ReadDir(sshDir)
	if err != nil {
		t.Fatalf("Couldn't read files from ssh dir")
	}

	var hashesAfter []sshKeyHash
	for _, file := range sshfiles {
		if !strings.HasSuffix(file.Name(), "_key.pub") {
			continue
		}
		hash, err := md5Sum(sshDir + file.Name())
		if err != nil {
			t.Fatalf("Couldn't hash file: %v", err)
		}
		hashesAfter = append(hashesAfter, sshKeyHash{file, hash})
	}

	if len(hashes) != len(hashesAfter) {
		t.Fatalf("Hashes changed after restarting guest agent")
	}

	for i := 0; i < len(hashes); i++ {
		if hashes[i].file.Name() != hashesAfter[i].file.Name() ||
			hashes[i].hash != hashesAfter[i].hash {
			t.Fatalf("Hashes changed after restarting guest agent")
		}
	}
}
