package hotattach

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
var Name = "hotattach"

const (
	diskName     = "hotattachMount"
	instanceName = "hotattachvm"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	// The extra scope is required to call detachDisk and attachDisk.
	hotattachParams := map[string]string{"machineType": "n2-standard-8", "extraScopes": "https://www.googleapis.com/auth/cloud-platform"}
	vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: instanceName, Type: imagetest.PdBalanced, SizeGb: 10}, {Name: diskName, Type: imagetest.PdBalanced, SizeGb: 30}}, hotattachParams)
	if err != nil {
		return err
	}
	vm.RunTests("TestFileHotAttach")
	return nil
}
