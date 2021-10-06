// +build cit

package startup_shutdown_script

import (
	"io/ioutil"
	"os/exec"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// TestStartupScript verify that the standard metadata script could run successfully
// by checking the output content.
func TestStartupScript(t *testing.T) {
	bytes, err := ioutil.ReadFile(startupOutputPath)
	if err != nil {
		t.Fatalf("failed to read startup script output %v", err)
	}
	output := strings.TrimSpace(string(bytes))
	if output != startupContent {
		t.Fatalf(`startup script output expect "%s", but actually "%s"`, startupOutputPath, output)
	}
}

// TestStartupScriptFailed test that a script failed execute doesn't crash the vm.
func TestStartupScriptFailed(t *testing.T) {
	if _, err := utils.GetMetadataAttribute("startup-script"); err != nil {
		t.Fatalf("couldn't get startup-script from metadata, %v", err)
	}
}

// TestDaemonScript test that daemon process started by startup script is still
// running in the VM after execution of startup script
func TestDaemonScript(t *testing.T) {
	bytes, err := ioutil.ReadFile(daemonOutputPath)
	if err != nil {
		t.Fatalf("failed to read deamon script output %v", err)
	}
	pid := string(bytes)
	cmd := exec.Command("ps", "-p", pid)
	if err := cmd.Run(); err != nil {
		t.Fatal("daemon script stop running")
	}
}
