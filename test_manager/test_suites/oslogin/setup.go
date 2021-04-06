package oslogin

import (
	"errors"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/testmanager"
)

var Name = "oslogin"

func TestSetup(t *testmanager.TestWorkflow) error {
	if strings.Contains(t.Image, "centos-7") {
		t.Skip("Not supported on CentOS-7")
		return nil
	}
	if strings.Contains(t.Image, "debian") {
		return errors.New("No debian!")
	}
	return testmanager.SingleVMTest(t)
}
