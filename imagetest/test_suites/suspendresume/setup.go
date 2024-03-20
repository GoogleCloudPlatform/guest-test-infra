package suspendresume

import (
	"strings"

	"github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "suspendresume"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if !strings.Contains(t.Image.Name, "rhel-8-2-sap") && !strings.Contains(t.Image.Name, "rhel-8-1-sap") && !strings.Contains(t.Image.Name, "debian-10") && !strings.Contains(t.Image.Family, "ubuntu-pro-1804-lts-arm64") {
		suspend := &daisy.Instance{}
		suspend.Scopes = append(suspend.Scopes, "https://www.googleapis.com/auth/cloud-platform")
		suspendvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "suspend"}}, suspend)
		if err != nil {
			return err
		}
		suspendvm.RunTests("TestSuspend")
		suspendvm.Resume()
	}
	return nil
}
