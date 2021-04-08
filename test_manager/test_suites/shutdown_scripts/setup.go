package shutdown_scripts

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/testmanager"
)

var (
	Name           = "shutdown-scripts"
	shutdownscript = `#!/bin/bash
count=1
while true; do
  echo $count | tee -a /root/the_log
  sync
  ((count++))
  sleep 1
done`
	timerfile = "/root/the_log"
)

func TestSetup(t *testmanager.TestWorkflow) error {
	vm1, err := t.CreateTestVM("vm")
	if err != nil {
		return err
	}
	vm1.SetShutdownScript(shutdownscript)
	vm1.Reboot()
	return nil
}
