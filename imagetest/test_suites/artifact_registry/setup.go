package artifactregistry

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "artifact_registry"

const (
	vmName = "vm"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	_, err := t.CreateTestVM(vmName)
	if err != nil {
		return err
	}
	return nil
}
