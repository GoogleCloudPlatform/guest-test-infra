package testmanager

import (
	"fmt"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
)

const (
	createVMsStepName   = "create-vms"
	createDisksStepName = "create-disks"
	successMatch        = "FINISHED-TEST"
)

// SingleVMTest configures the simple test case of one VM running the test
// package.
func SingleVMTest(t *TestWorkflow) error {
	_, err := t.CreateTestVM(t.Name)
	return err
}

// Disable disables a workflow.
func (t *TestWorkflow) Disable() {
	t.wf = nil
}

// Skip marks a test workflow to be skipped.
func (t *TestWorkflow) Skip(message string) {
	// TODO: figure out where to put the message
	t.skipped = true
}

// CreateTestVM creates the necessary steps to create a VM with the specified name to the workflow.
func (t *TestWorkflow) CreateTestVM(name string) (*TestVM, error) {

	createDisksStep, err := t.addCreateDisksStep(name)
	if err != nil {
		return nil, err
	}

	// createDisksStep doesn't depend on any other steps.

	createVMStep, err := t.addCreateVMStep(name)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return nil, err
	}

	waitStep, err := t.addWaitStep("wait-vm-"+name, name, false)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	return &TestVM{name: name, testWorkflow: t}, nil
}

// TestVM is a test VM.
type TestVM struct {
	name         string
	testWorkflow *TestWorkflow
	instance     *daisy.Instance
}

// AddMetadata adds the specified key:value pair to metadata during VM creation.
func (t *TestVM) AddMetadata(key, value string) {
	createVMStep := t.testWorkflow.wf.Steps[createVMsStepName]
	for _, vm := range createVMStep.CreateInstances.Instances {
		if vm.Name == t.name {
			if vm.Metadata == nil {
				vm.Metadata = make(map[string]string)
			}
			vm.Metadata[key] = value
			return
		}
	}
}

// RunTests runs only the named tests on the testVM.
//
//    -run regexp
//     Run only those tests and examples matching the regular expression.
//     For tests, the regular expression is split by unbracketed slash (/)
//     characters into a sequence of regular expressions, and each part
//     of a test's identifier must match the corresponding element in
//     the sequence, if any. Note that possible parents of matches are
//     run too, so that -run=X/Y matches and runs and reports the result
//     of all tests matching X, even those without sub-tests matching Y,
//     because it must run them to look for those sub-tests.
func (t *TestVM) RunTests(runtest string) {
	t.AddMetadata("_test_run", runtest)
}

// SetShutdownScript sets the `shutdown-script` metadata key for a VM.
func (t *TestVM) SetShutdownScript(script string) {
	t.AddMetadata("shutdown-script", script)
}

// SetStartupScript sets the `startup-script` metadata key for a VM.
func (t *TestVM) SetStartupScript(script string) {
	t.AddMetadata("startup-script", script)
}

// Reboot stops the VM, waits for it to shutdown, then starts it again. Your
// test package must handle being run twice.
func (t *TestVM) Reboot() error {
	// appends to existing "wait-vm-name" step. Should look like:
	// "wait-vm-name" => "stop-name" => "wait-stopped-name" => "start-name" => "wait-started-name"

	waitStep, ok := t.testWorkflow.wf.Steps["wait-vm-"+t.name]
	if !ok {
		return fmt.Errorf("wait-vm-%s step missing", t.name)
	}

	stopInstancesStep, err := t.testWorkflow.addStopStep("stop-" + t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(stopInstancesStep, waitStep); err != nil {
		return err
	}

	waitStopStep, err := t.testWorkflow.addWaitStep("wait-stopped-"+t.name, t.name, true)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStopStep, stopInstancesStep); err != nil {
		return err
	}

	startInstancesStep, err := t.testWorkflow.addStartStep("start-" + t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(startInstancesStep, waitStopStep); err != nil {
		return err
	}

	waitStartedStep, err := t.testWorkflow.addWaitStep("wait-started-"+t.name, t.name, false)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStartedStep, startInstancesStep); err != nil {
		return err
	}

	return nil
}
