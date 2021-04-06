package testmanager

import (
	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"google.golang.org/api/compute/v1"
)

// SingleVMTest configures the simple test case of one VM running the test
// package.
func SingleVMTest(t *TestWorkflow) error {
	_, err := t.CreateTestVM(t.Name)
	return err
}

// TestVM is a test VM.
type TestVM struct {
	name         string
	testWorkflow *TestWorkflow
	instance     *daisy.Instance
}

// CreateTestVM adds steps to a workflow to create a test VM. The workflow is
// created if it doesn't exist. The first VM created has a WaitInstances step
// configured.
func (t *TestWorkflow) CreateTestVM(name string) (*TestVM, error) {
	testVM := &TestVM{name: name, testWorkflow: t}

	if t.wf == nil {
		t.wf = daisy.New()
		t.wf.Name = t.Name
		if err := t.setupBaseWorkflow(name); err != nil {
			return nil, err
		}
		return testVM, nil
	}

	// Test VMs and disks are created in parallel, so we're just appending.
	bootdisk := &daisy.Disk{}
	bootdisk.Name = name
	bootdisk.SourceImage = t.Image
	createDisksStep := t.wf.Steps["create-disks"]
	*createDisksStep.CreateDisks = append(*createDisksStep.CreateDisks, bootdisk)

	instance := &daisy.Instance{}
	instance.StartupScript = "startup"
	instance.Name = name
	instance.Scopes = append(instance.Scopes, "https://www.googleapis.com/auth/devstorage.read_write")

	attachedDisk := &compute.AttachedDisk{Source: name}
	instance.Disks = append(instance.Disks, attachedDisk)

	createVMStep := t.wf.Steps["create-vms"]
	createVMStep.CreateInstances.Instances = append(createVMStep.CreateInstances.Instances, instance)

	return testVM, nil
}

// TODO: break these two into smaller functions for each step and add unit tests.
//       setupBase should not add the first VM, but the workflow itself. A test
//       author may first call createnetwork, or similar.

func (t *TestWorkflow) setupBaseWorkflow(name string) error {
	bootdisk := &daisy.Disk{}
	bootdisk.Name = name
	bootdisk.SourceImage = t.Image
	createDisks := &daisy.CreateDisks{bootdisk}
	createDisksStep, err := t.wf.NewStep("create-disks")
	if err != nil {
		return err
	}
	createDisksStep.CreateDisks = createDisks

	instance := &daisy.Instance{}
	instance.StartupScript = "startup"
	instance.Name = name
	instance.Scopes = append(instance.Scopes, "https://www.googleapis.com/auth/devstorage.read_write")

	attachedDisk := &compute.AttachedDisk{Source: name}
	instance.Disks = append(instance.Disks, attachedDisk)

	createInstances := &daisy.CreateInstances{}
	createInstances.Instances = append(createInstances.Instances, instance)

	createVMStep, err := t.wf.NewStep("create-vms")
	if err != nil {
		return err
	}
	_ = t.wf.Steps["create-vms"]
	createVMStep.CreateInstances = createInstances
	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return err
	}

	instanceSignal := &daisy.InstanceSignal{}
	instanceSignal.Name = name
	// TODO: implement waiting on guest attributes in daisy.
	serialOutput := &daisy.SerialOutput{}
	serialOutput.Port = 1
	serialOutput.SuccessMatch = "MAGIC-STRING"
	instanceSignal.SerialOutput = serialOutput
	waitForInstances := &daisy.WaitForInstancesSignal{instanceSignal}

	waitStep, err := t.wf.NewStep("wait-" + name)
	if err != nil {
		return err
	}
	waitStep.WaitForInstancesSignal = waitForInstances

	t.wf.AddDependency(waitStep, createVMStep)

	copyGCSObject := daisy.CopyGCSObject{}
	copyGCSObject.Source = "${OUTSPATH}/junit.xml"
	copyGCSObjects := &daisy.CopyGCSObjects{copyGCSObject}
	copyStep, err := t.wf.NewStep("copy-objects")
	if err != nil {
		return err
	}
	copyStep.CopyGCSObjects = copyGCSObjects

	t.wf.AddDependency(copyStep, waitStep)

	return nil
}

// Skip marks a test workflow to be skipped.
func (t *TestWorkflow) Skip(message string) {
	// TODO: figure out where to put the message
	t.skipped = true
}

// AddMetadata adds the specified key:value pair to metadata during VM creation.
func (t *TestVM) AddMetadata(key, value string) {
	createVMStep := t.testWorkflow.wf.Steps["create-vms"]
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

// AddWait adds a daisy.WaitForInstancesSignal Step depending on this VM. Note:
// the first created VM automatically has a wait step created.
func (t *TestVM) AddWait(success, failure, status string, stopped bool) error {
	/*
		if _, ok := t.testWorkflow.wf.Steps["wait-"+t.name]; ok {
			return fmt.Errorf("wait step already exists for TestVM %q", t.name)
		}
	*/
	instanceSignal := &daisy.InstanceSignal{}
	instanceSignal.Name = t.name
	instanceSignal.Stopped = stopped
	if success != "" {
		instanceSignal.SerialOutput.SuccessMatch = success
	}
	if status != "" {
		instanceSignal.SerialOutput.StatusMatch = status
	}
	if failure != "" {
		// FailureMatch is a []string, compared to success and status.
		instanceSignal.SerialOutput.FailureMatch = append(instanceSignal.SerialOutput.FailureMatch, failure)
	}
	waitForInstances := &daisy.WaitForInstancesSignal{instanceSignal}
	s, err := t.testWorkflow.wf.NewStep("wait-" + t.name)
	if err != nil {
		return err
	}
	s.WaitForInstancesSignal = waitForInstances
	t.testWorkflow.wf.AddDependency(s, t.testWorkflow.wf.Steps["create-vms"])
	return nil
}

// Reboot stops the VM, waits for it to shutdown, then starts it again. Your
// test package must handle being run twice.
func (t *TestVM) Reboot() error {
	instanceSignal := &daisy.InstanceSignal{}
	instanceSignal.Name = t.name
	serialOutput := &daisy.SerialOutput{}
	serialOutput.Port = 1
	serialOutput.SuccessMatch = "FINISHED-BOOTING"
	instanceSignal.SerialOutput = serialOutput
	waitForInstances := &daisy.WaitForInstancesSignal{instanceSignal}

	waitBootStep, err := t.testWorkflow.wf.NewStep("wait-boot-" + t.name)
	if err != nil {
		return err
	}
	waitBootStep.WaitForInstancesSignal = waitForInstances

	t.testWorkflow.wf.AddDependency(waitBootStep, t.testWorkflow.wf.Steps["create-vms"])

	stopInstances := &daisy.StopInstances{}
	stopInstances.Instances = append(stopInstances.Instances, t.name)

	stopInstancesStep, err := t.testWorkflow.wf.NewStep("stop-" + t.name)
	if err != nil {
		return err
	}
	stopInstancesStep.StopInstances = stopInstances

	t.testWorkflow.wf.AddDependency(stopInstancesStep, waitBootStep)

	instanceSignalStop := &daisy.InstanceSignal{}
	instanceSignalStop.Name = t.name
	instanceSignalStop.Stopped = true
	waitForInstancesStop := &daisy.WaitForInstancesSignal{instanceSignalStop}

	waitStopStep, err := t.testWorkflow.wf.NewStep("wait-stopped-" + t.name)
	if err != nil {
		return err
	}
	waitStopStep.WaitForInstancesSignal = waitForInstancesStop

	t.testWorkflow.wf.AddDependency(waitStopStep, stopInstancesStep)

	startInstances := &daisy.StartInstances{}
	startInstances.Instances = append(startInstances.Instances, t.name)

	startInstancesStep, err := t.testWorkflow.wf.NewStep("start-" + t.name)
	if err != nil {
		return err
	}
	startInstancesStep.StartInstances = startInstances

	t.testWorkflow.wf.AddDependency(startInstancesStep, waitStopStep)

	// Make original wait step also wait on the outcome of the reboot.
	t.testWorkflow.wf.AddDependency(t.testWorkflow.wf.Steps["wait-"+t.name], startInstancesStep)

	return nil
}
