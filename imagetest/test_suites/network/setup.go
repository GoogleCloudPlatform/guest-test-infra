package network

import "github.com/GoogleCloudPlatform/guest-test-infra/imagetest"

// Name is the name of the test package. It must match the directory name.
var Name = "network"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	linux, err := t.CreateTestVM("linux")
	if err != nil {
		return err
	}
	linux.RunTests("TestDebianDefaultMTU")

	debian, err := t.CreateTestVM("debian")
	if err != nil {
		return err
	}
	debian.RunTests("TestDebianDefaultMTU")

	windows2008, err := t.CreateTestVM("windows2008")
	if err != nil {
		return err
	}
	windows2008.RunTests("TestWindows2008DefaultMTU")

	windows2008beyond, err := t.CreateTestVM("windows2008beyond")
	if err != nil {
		return err
	}
	windows2008beyond.RunTests("TestWin2012BeyondDefaultMTU")
	return nil
}
