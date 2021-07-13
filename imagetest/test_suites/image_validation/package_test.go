package imagevalidation

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestStandardPrograms(t *testing.T) {
	cmd := exec.Command("gcloud", "-h")
	if err := cmd.Run(); err != nil {
		t.Fatalf("gcloud not installed properly")
	}
	cmd = exec.Command("gsutil", "help")
	if err := cmd.Run(); err != nil {
		t.Fatalf("gsutil not installed properly")
	}
}

func TestGuestPackages(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't determine image from metadata")
	}
	cmdPrefix := []string{"rpm", "-q", "--queryformat", "'%{NAME}\n'"}
	if strings.Contains(image, "debian") {
		cmdPrefix = []string{"dpkg-query", "-W", "--showformat", "'${Package}\n'"}
	}
	for _, pkg := range []string{"google-compute-engine",
		"google-compute-engine-oslogin", "google-guest-agent",
		"google-osconfig-agent"} {
		args := append(cmdPrefix[1:], pkg)
		cmd := exec.Command(cmdPrefix[0], args...)
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr
		stdout := &bytes.Buffer{}
		cmd.Stdout = stdout
		if err := cmd.Run(); err != nil {
			t.Errorf("error running check command: %v %s %s", err, stdout, stderr)
			continue
		}
	}
}

// TestAppArmor: Verify that AppArmor is working correctly.
func TestAppArmor(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't determine image from metadata")
	}
	if strings.Contains(image, "ubuntu") {
		cmd := exec.Command("/usr/sbin/aa-status")
		if err := cmd.Run(); err != nil {
			t.Fatal("AppArmor is not working correctly")
		}
	}
}
