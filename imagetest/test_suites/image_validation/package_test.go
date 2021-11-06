// +build cit

package imagevalidation

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestStandardPrograms(t *testing.T) {
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}
	if strings.Contains(image, "sles") {
		// SLES does not have the Google Cloud SDK installed.
		t.Skip("Not supported on SLES")
	}

	cmd := exec.Command("gcloud", "-h")
	cmd.Start()
	err := cmd.Wait()
	if err != nil {
		t.Fatalf("gcloud not installed properly")
	}
	cmd = exec.Command("gsutil", "help")
	cmd.Start()
	err = cmd.Wait()
	if err != nil {
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
		if err := cmd.Start(); err != nil {
			t.Errorf("error starting check command: %v", err)
			continue
		}
		if err := cmd.Wait(); err != nil {
			t.Errorf("error running check command: %v %s %s", err, stdout, stderr)
			continue
		}

	}

}
