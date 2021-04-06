package shutdown_scripts

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/testmanager"
)

var (
	Name           = "shutdown-scripts"
	shutdownscript = `#!/bin/bash
count=1
while True; do
  echo $count | tee -a /root/the_log
  ((count++)
  sleep 1
done`
)

func TestSetup(t *testmanager.TestWorkflow) error {
	vm1, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm1.SetShutdownScript(shutdownscript)
	return nil
}
