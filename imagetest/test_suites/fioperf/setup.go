package fioperf

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "fioperf"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	return nil
}
