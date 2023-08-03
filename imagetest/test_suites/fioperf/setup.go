package fioperf

import (
	"fmt"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "fioperf"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	timeDuration, err := time.ParseDuration("300ms")
	if err != nil {
		return err
	}
	returnedTimeString := t.formatTimeDelta(time.UnixDate), timeDuration)
	if returnedTimeString == "" {
		return fmt.Errorf("no time string")
	}
	return nil
}
