// Package livemigrate tests standard live migration, not confidential vm live migrate. See the cvm suite for the latter.
package livemigrate

import (
	"github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "livemigrate"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	lm := &daisy.Instance{}
	lm.Scopes = append(lm.Scopes, "https://www.googleapis.com/auth/cloud-platform")
	lm.Scheduling = &compute.Scheduling{OnHostMaintenance: "MIGRATE"}
	lmvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "livemigrate"}}, lm)
	if err != nil {
		return err
	}
	lmvm.RunTests("TestLiveMigrate")
	return nil
}
