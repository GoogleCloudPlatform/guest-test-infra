// +build cit

package preinstalled

import (
	"os/exec"
	"testing"
)

func TestStandardPrograms(t *testing.T) {
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

