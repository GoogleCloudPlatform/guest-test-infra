// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imagetest

import (
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
)

const (
	createVMsStepName   = "create-vms"
	createDisksStepName = "create-disks"
	successMatch        = "FINISHED-TEST"
)

// SingleVMTest configures one VM running tests.
func SingleVMTest(t *TestWorkflow) error {
	_, err := t.CreateTestVM("vm")
	return err
}

// Skip marks a test workflow to be skipped.
func (t *TestWorkflow) Skip(message string) {
	t.skipped = true
	t.skippedMessage = message
}

// SkippedMessage returns the skip reason message for the workflow.
func (t *TestWorkflow) SkippedMessage() string {
	return t.skippedMessage
}

// CreateTestVM creates the necessary steps to create a VM with the specified name to the workflow.
func (t *TestWorkflow) CreateTestVM(name string) (*TestVM, error) {
	// TODO: more robust name validation.
	name = strings.ReplaceAll(name, "_", "-")

	createDisksStep, err := t.appendCreateDisksStep(name)
	if err != nil {
		return nil, err
	}

	// createDisksStep doesn't depend on any other steps.

	createVMStep, err := t.appendCreateVMStep(name)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return nil, err
	}

	waitStep, err := t.addWaitStep(name, name, false)
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
// From go help test:
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
	// Grab the wait step that was added with CreateTestVM.
	waitStep, ok := t.testWorkflow.wf.Steps["wait-"+t.name]
	if !ok {
		return fmt.Errorf("wait-%s step missing", t.name)
	}

	stopInstancesStep, err := t.testWorkflow.addStopStep(t.name, t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(stopInstancesStep, waitStep); err != nil {
		return err
	}

	waitStopStep, err := t.testWorkflow.addWaitStep("stopped-"+t.name, t.name, true)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStopStep, stopInstancesStep); err != nil {
		return err
	}

	startInstancesStep, err := t.testWorkflow.addStartStep(t.name, t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(startInstancesStep, waitStopStep); err != nil {
		return err
	}

	waitStartedStep, err := t.testWorkflow.addWaitStep("started-"+t.name, t.name, false)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStartedStep, startInstancesStep); err != nil {
		return err
	}

	return nil
}
