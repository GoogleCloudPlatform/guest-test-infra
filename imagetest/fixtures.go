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
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/google/uuid"
	computeBeta "google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/compute/v1"
)

const (
	waitForVMQuotaStepName    = "wait-for-vm-quota"
	createVMsStepName         = "create-vms"
	createDisksStepName       = "create-disks"
	waitForDisksQuotaStepName = "wait-for-disk-quota"
	createNetworkStepName     = "create-networks"
	createFirewallStepName    = "create-firewalls"
	createSubnetworkStepName  = "create-sub-networks"
	successMatch              = "FINISHED-TEST"
	// ShouldRebootDuringTest is a local map key to indicate that the
	// test will reboot and relies on results from the second boot.
	ShouldRebootDuringTest = "shouldRebootDuringTest"
	// DefaultSourceRange is the RFC-1918 range used in default rules.
	DefaultSourceRange = "10.128.0.0/9"

	// DefaultMTU is the default MTU set for a network.
	DefaultMTU = 1460

	// JumboFramesMTU is the maximum MTU settable for a network.
	JumboFramesMTU = 8896

	// DefaultMachineType is the default machine type when machine type isn't specified.
	DefaultMachineType = "n1-standard-1"
)

// TestVM is a test VM.
type TestVM struct {
	name         string
	testWorkflow *TestWorkflow
	// The underlying instance running the test. Exactly one of these must be non-nil.
	instance     *daisy.Instance
	instancebeta *daisy.InstanceBeta
}

// AddUser add user public key to metadata ssh-keys.
func (t *TestVM) AddUser(user, publicKey string) {
	keyline := fmt.Sprintf("%s:%s", user, publicKey)
	if t.instance != nil {
		if keys, ok := t.instance.Metadata["ssh-keys"]; ok {
			keyline = fmt.Sprintf("%s\n%s", keys, keyline)
		}
	} else if t.instancebeta != nil {
		if keys, ok := t.instancebeta.Metadata["ssh-keys"]; ok {
			keyline = fmt.Sprintf("%s\n%s", keys, keyline)
		}
	}
	t.AddMetadata("ssh-keys", keyline)
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

// LockProject indicates this test modifies project-level data and must have
// exclusive use of the project.
func (t *TestWorkflow) LockProject() {
	t.lockProject = true
}

// WaitForVMQuota appends a list of quotas to the wait for vm quota step. Quotas with a blank region will be populated with the region corresponding to the workflow zone.
func (t *TestWorkflow) WaitForVMQuota(qa *daisy.QuotaAvailable) error {
	return t.waitForQuotaStep(qa, waitForVMQuotaStepName)
}

// WaitForDisksQuota appends a list of quotas to the wait for disk quota step. Quotas with a blank region will be populated with the region corresponding to the workflow zone.
func (t *TestWorkflow) WaitForDisksQuota(qa *daisy.QuotaAvailable) error {
	return t.waitForQuotaStep(qa, waitForDisksQuotaStepName)
}

func (t *TestWorkflow) waitForQuotaStep(qa *daisy.QuotaAvailable, stepname string) error {
	step, ok := t.wf.Steps[stepname]
	if !ok {
		var err error
		step, err = t.wf.NewStep(stepname)
		if err != nil {
			return err
		}
		step.WaitForAvailableQuotas = &daisy.WaitForAvailableQuotas{Interval: "90s"}
	}
	// If the step is already waiting for this quota, add the number of units to
	// the existing quota.
	for _, q := range step.WaitForAvailableQuotas.Quotas {
		if q.Metric == qa.Metric && q.Region == qa.Region {
			q.Units = q.Units + qa.Units
			return nil
		}
	}
	step.WaitForAvailableQuotas.Quotas = append(step.WaitForAvailableQuotas.Quotas, qa)
	return nil
}

// CreateTestVM adds the necessary steps to create a VM with the specified
// name to the workflow.
func (t *TestWorkflow) CreateTestVM(name string) (*TestVM, error) {
	parts := strings.Split(name, ".")
	vmname := strings.ReplaceAll(parts[0], "_", "-")

	bootDisk := &compute.Disk{Name: vmname}
	createDisksStep, err := t.appendCreateDisksStep(bootDisk)
	if err != nil {
		return nil, err
	}

	daisyInst := &daisy.Instance{}
	// createDisksStep doesn't depend on any other steps.
	createVMStep, i, err := t.appendCreateVMStep([]*compute.Disk{bootDisk}, daisyInst)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return nil, err
	}

	waitStep, err := t.addWaitStep(vmname, vmname)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	if createSubnetworkStep, ok := t.wf.Steps[createSubnetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createSubnetworkStep); err != nil {
			return nil, err
		}
	}

	if createNetworkStep, ok := t.wf.Steps[createNetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createNetworkStep); err != nil {
			return nil, err
		}
	}

	return &TestVM{name: vmname, testWorkflow: t, instance: i}, nil
}

// CreateTestVMBeta adds the necessary steps to create a VM with the specified
// name from the compute beta API to the workflow.
func (t *TestWorkflow) CreateTestVMBeta(name string) (*TestVM, error) {
	parts := strings.Split(name, ".")
	vmname := strings.ReplaceAll(parts[0], "_", "-")

	bootDisk := &compute.Disk{Name: vmname}
	createDisksStep, err := t.appendCreateDisksStep(bootDisk)
	if err != nil {
		return nil, err
	}

	daisyInst := &daisy.InstanceBeta{}
	// createDisksStep doesn't depend on any other steps.
	createVMStep, i, err := t.appendCreateVMStepBeta([]*compute.Disk{bootDisk}, daisyInst)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return nil, err
	}

	waitStep, err := t.addWaitStep(vmname, vmname)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	if createSubnetworkStep, ok := t.wf.Steps[createSubnetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createSubnetworkStep); err != nil {
			return nil, err
		}
	}

	if createNetworkStep, ok := t.wf.Steps[createNetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createNetworkStep); err != nil {
			return nil, err
		}
	}

	return &TestVM{name: vmname, testWorkflow: t, instancebeta: i}, nil
}

// CreateTestVMMultipleDisks adds the necessary steps to create a VM with the specified
// name to the workflow.
func (t *TestWorkflow) CreateTestVMMultipleDisks(disks []*compute.Disk, instanceParams *daisy.Instance) (*TestVM, error) {
	if len(disks) == 0 || disks[0].Name == "" {
		return nil, fmt.Errorf("failed to create multiple disk VM with empty boot disk")
	}

	name := disks[0].Name
	parts := strings.Split(name, ".")
	vmname := strings.ReplaceAll(parts[0], "_", "-")

	createDisksSteps := make([]*daisy.Step, len(disks))
	for i, disk := range disks {
		// the disk creation steps are slightly different for the boot disk and mount disks
		var createDisksStep *daisy.Step
		var err error
		if i == 0 {
			createDisksStep, err = t.appendCreateDisksStep(disk)
		} else {
			createDisksStep, err = t.appendCreateMountDisksStep(disk)
		}

		if err != nil {
			return nil, err
		}
		createDisksSteps[i] = createDisksStep
	}
	var daisyInst *daisy.Instance
	if instanceParams == nil {
		daisyInst = &daisy.Instance{}
	} else {
		daisyInst = instanceParams
	}
	// createDisksStep doesn't depend on any other steps.
	createVMStep, i, err := t.appendCreateVMStep(disks, daisyInst)
	if err != nil {
		return nil, err
	}
	for _, createDisksStep := range createDisksSteps {
		if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
			return nil, err
		}
	}

	// In a follow-up, guest attribute support will be added.
	// If this is the first boot before a reboot, this should use a
	// different guest attribute when waiting for the instance signal.
	var waitStep *daisy.Step
	if _, foundKey := daisyInst.Metadata[ShouldRebootDuringTest]; foundKey {
		waitStep, err = t.addWaitRebootGAStep(vmname, vmname)
	} else {
		waitStep, err = t.addWaitStep(vmname, vmname)
	}
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	if createSubnetworkStep, ok := t.wf.Steps[createSubnetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createSubnetworkStep); err != nil {
			return nil, err
		}
	}

	if createNetworkStep, ok := t.wf.Steps[createNetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createNetworkStep); err != nil {
			return nil, err
		}
	}

	return &TestVM{name: vmname, testWorkflow: t, instance: i}, nil
}

// CreateTestVMFromInstanceBeta creates a test vm struct to run CIT suites on from
// the given daisy instancebeta and adds it to the test workflow.
func (t *TestWorkflow) CreateTestVMFromInstanceBeta(i *daisy.InstanceBeta, disks []*compute.Disk) (*TestVM, error) {
	if len(disks) == 0 || disks[0].Name == "" {
		return nil, fmt.Errorf("failed to create multiple disk VM with empty boot disk")
	}

	name := disks[0].Name
	parts := strings.Split(name, ".")
	vmname := strings.ReplaceAll(parts[0], "_", "-")

	createDisksSteps := make([]*daisy.Step, len(disks))
	for i, disk := range disks {
		// the disk creation steps are slightly different for the boot disk and mount disks
		var createDisksStep *daisy.Step
		var err error
		if i == 0 {
			createDisksStep, err = t.appendCreateDisksStep(disk)
		} else {
			createDisksStep, err = t.appendCreateMountDisksStep(disk)
		}

		if err != nil {
			return nil, err
		}
		createDisksSteps[i] = createDisksStep
	}
	// createDisksStep doesn't depend on any other steps.
	createVMStep, i, err := t.appendCreateVMStepBeta(disks, i)
	if err != nil {
		return nil, err
	}
	for _, createDisksStep := range createDisksSteps {
		if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
			return nil, err
		}
	}

	var waitStep *daisy.Step
	if _, foundKey := i.Metadata[ShouldRebootDuringTest]; foundKey {
		waitStep, err = t.addWaitRebootGAStep(vmname, vmname)
	} else {
		waitStep, err = t.addWaitStep(vmname, vmname)
	}
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	if createSubnetworkStep, ok := t.wf.Steps[createSubnetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createSubnetworkStep); err != nil {
			return nil, err
		}
	}

	if createNetworkStep, ok := t.wf.Steps[createNetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createNetworkStep); err != nil {
			return nil, err
		}
	}

	return &TestVM{name: vmname, testWorkflow: t, instancebeta: i}, nil
}

// AddMetadata adds the specified key:value pair to metadata during VM creation.
func (t *TestVM) AddMetadata(key, value string) {
	if t.instance != nil {
		if t.instance.Metadata == nil {
			t.instance.Metadata = make(map[string]string)
		}
		t.instance.Metadata[key] = value
	} else if t.instancebeta != nil {
		if t.instancebeta.Metadata == nil {
			t.instancebeta.Metadata = make(map[string]string)
		}
		t.instancebeta.Metadata[key] = value
	}
}

// AddScope adds the specified auth scope to the service account on the VM.
func (t *TestVM) AddScope(scope string) {
	if t.instance != nil {
		t.instance.Scopes = append(t.instance.Scopes, scope)
	} else if t.instancebeta != nil {
		t.instancebeta.Scopes = append(t.instancebeta.Scopes, scope)
	}
}

// RunTests runs only the named tests on the testVM.
//
// From go help test:
//
//	-run regexp
//	 Run only those tests and examples matching the regular expression.
//	 For tests, the regular expression is split by unbracketed slash (/)
//	 characters into a sequence of regular expressions, and each part
//	 of a test's identifier must match the corresponding element in
//	 the sequence, if any. Note that possible parents of matches are
//	 run too, so that -run=X/Y matches and runs and reports the result
//	 of all tests matching X, even those without sub-tests matching Y,
//	 because it must run them to look for those sub-tests.
func (t *TestVM) RunTests(runtest string) {
	t.AddMetadata("_test_run", runtest)
}

// SetShutdownScript sets the `shutdown-script` metadata key for a non-Windows VM.
func (t *TestVM) SetShutdownScript(script string) {
	t.AddMetadata("shutdown-script", script)
}

// SetWindowsShutdownScript sets the `windows-shutdown-script-ps1` metadata key for a Windows VM.
func (t *TestVM) SetWindowsShutdownScript(script string) {
	t.AddMetadata("windows-shutdown-script-ps1", script)
}

// SetShutdownScriptURL sets the`shutdown-script-url` metadata key for a non-Windows VM.
func (t *TestVM) SetShutdownScriptURL(script string) error {
	fileName := fmt.Sprintf("/shutdown_script-%s", uuid.New())
	if err := ioutil.WriteFile(fileName, []byte(script), 0755); err != nil {
		return err
	}
	t.testWorkflow.wf.Sources["shutdown-script"] = fileName

	t.AddMetadata("shutdown-script-url", "${SOURCESPATH}/shutdown-script")
	return nil
}

// SetWindowsShutdownScriptURL sets the`windows-shutdown-script-url` metadata key for a Windows VM.
func (t *TestVM) SetWindowsShutdownScriptURL(script string) error {
	fileName := fmt.Sprintf("/shutdown_script-%s.ps1", uuid.New())
	if err := ioutil.WriteFile(fileName, []byte(script), 0755); err != nil {
		return err
	}
	t.testWorkflow.wf.Sources["shutdown-script.ps1"] = fileName

	t.AddMetadata("windows-shutdown-script-url", "${SOURCESPATH}/shutdown-script.ps1")
	return nil
}

// SetStartupScript sets the `startup-script` metadata key for a VM. On Windows VMs, this script does not run on startup: different guest attributes must be set.
func (t *TestVM) SetStartupScript(script string) {
	t.AddMetadata("startup-script", script)
}

// SetWindowsStartupScript sets the `windows-startup-script-ps1` metadata key for a VM.
func (t *TestVM) SetWindowsStartupScript(script string) {
	t.AddMetadata("windows-startup-script-ps1", script)
}

// SetNetworkPerformanceTier sets the performance tier of the VM.
// The tier must be one of "DEFAULT" or "TIER_1"
func (t *TestVM) SetNetworkPerformanceTier(tier string) error {
	if tier != "DEFAULT" && tier != "TIER_1" {
		return fmt.Errorf("Error: %v not one of DEFAULT or TIER_1", tier)
	}
	if t.instance != nil {
		if t.instance.NetworkPerformanceConfig == nil {
			t.instance.NetworkPerformanceConfig = &compute.NetworkPerformanceConfig{TotalEgressBandwidthTier: tier}
		} else {
			t.instance.NetworkPerformanceConfig.TotalEgressBandwidthTier = tier
		}
	} else if t.instancebeta != nil {
		if t.instancebeta.NetworkPerformanceConfig == nil {
			t.instancebeta.NetworkPerformanceConfig = &computeBeta.NetworkPerformanceConfig{TotalEgressBandwidthTier: tier}
		} else {
			t.instancebeta.NetworkPerformanceConfig.TotalEgressBandwidthTier = tier
		}
	}
	return nil
}

// Reboot stops the VM, waits for it to shutdown, then starts it again. Your
// test package must handle being run twice.
func (t *TestVM) Reboot() error {
	// TODO: better solution than a shared counter for name collisions.
	t.testWorkflow.counter++
	stepSuffix := fmt.Sprintf("%s-%d", t.name, t.testWorkflow.counter)

	lastStep, err := t.testWorkflow.getLastStepForVM(t.name)
	if err != nil {
		return fmt.Errorf("failed resolve last step")
	}

	stopInstancesStep, err := t.testWorkflow.addStopStep(stepSuffix, t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(stopInstancesStep, lastStep); err != nil {
		return err
	}

	waitStopStep, err := t.testWorkflow.addWaitStoppedStep("stopped-"+stepSuffix, t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStopStep, stopInstancesStep); err != nil {
		return err
	}

	startInstancesStep, err := t.testWorkflow.addStartStep(stepSuffix, t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(startInstancesStep, waitStopStep); err != nil {
		return err
	}

	waitStartedStep, err := t.testWorkflow.addWaitStep("started-"+stepSuffix, t.name)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStartedStep, startInstancesStep); err != nil {
		return err
	}
	return nil
}

// Resume waits for the vm to be SUSPENDED, then resumes it. It does not handle suspension.
func (t *TestVM) Resume() error {
	// TODO: better solution than a shared counter for name collisions.
	t.testWorkflow.counter++
	stepSuffix := fmt.Sprintf("%s-%d", t.name, t.testWorkflow.counter)

	lastStep, err := t.testWorkflow.getLastStepForVM(t.name)
	if err != nil {
		return fmt.Errorf("failed resolve last step")
	}

	waitSuspended, err := t.testWorkflow.wf.NewStep(fmt.Sprintf("wait-suspended-%s", stepSuffix))
	if err != nil {
		return err
	}
	waitSuspended.WaitForInstancesSignal = &daisy.WaitForInstancesSignal{
		{Name: t.name, Status: []string{"SUSPENDED"}},
	}

	createStep := t.testWorkflow.wf.Steps[createVMsStepName]

	if err := t.testWorkflow.wf.AddDependency(waitSuspended, createStep); err != nil {
		return err
	}

	resume, err := t.testWorkflow.wf.NewStep(fmt.Sprintf("resume-%s", stepSuffix))
	if err != nil {
		return err
	}
	resume.Resume = &daisy.Resume{
		Instance: t.name,
	}
	if err := t.testWorkflow.wf.AddDependency(resume, waitSuspended); err != nil {
		return err
	}
	if err := t.testWorkflow.wf.AddDependency(lastStep, resume); err != nil {
		return err
	}

	return nil
}

// ResizeDiskAndReboot resize the disk of the current test VMs and reboot
func (t *TestVM) ResizeDiskAndReboot(diskSize int) error {
	t.testWorkflow.counter++
	stepSuffix := fmt.Sprintf("%s-%d", t.name, t.testWorkflow.counter)

	lastStep, err := t.testWorkflow.getLastStepForVM(t.name)
	if err != nil {
		return fmt.Errorf("failed resolve last step")
	}

	diskResizeStep, err := t.testWorkflow.addDiskResizeStep(stepSuffix, t.name, diskSize)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(diskResizeStep, lastStep); err != nil {
		return err
	}

	return t.Reboot()
}

// ForceMachineType sets the machine type for the test VM. This will override
// the machine_type flag in the CIT wrapper, and should only be used when a
// test absolutely requires a specific machine shape.
func (t *TestVM) ForceMachineType(machinetype string) {
	if t.instance != nil {
		t.instance.MachineType = machinetype
	} else if t.instancebeta != nil {
		t.instancebeta.MachineType = machinetype
	}
}

// ForceZone sets the zone for the test vm. This will override the zone option
// from the CIT wrapper and and should only be used when a test requires a specific
// zone.
func (t *TestVM) ForceZone(z string) {
	if t.instance != nil {
		t.instance.Zone = z
	} else if t.instancebeta != nil {
		t.instancebeta.Zone = z
	}
}

// EnableSecureBoot make the current test VMs in workflow with secure boot.
func (t *TestVM) EnableSecureBoot() {
	if t.instance != nil {
		if t.instance.ShieldedInstanceConfig == nil {
			t.instance.ShieldedInstanceConfig = &compute.ShieldedInstanceConfig{}
		}
		t.instance.ShieldedInstanceConfig.EnableSecureBoot = true
	} else if t.instancebeta != nil {
		if t.instancebeta.ShieldedInstanceConfig == nil {
			t.instancebeta.ShieldedInstanceConfig = &computeBeta.ShieldedInstanceConfig{}
		}
		t.instancebeta.ShieldedInstanceConfig.EnableSecureBoot = true
	}
}

// EnableConfidentialInstance enabled CVM features for the instance.
func (t *TestVM) EnableConfidentialInstance() {
	if t.instance != nil {
		if t.instance.ConfidentialInstanceConfig == nil {
			t.instance.ConfidentialInstanceConfig = &compute.ConfidentialInstanceConfig{}
		}
		t.instance.ConfidentialInstanceConfig.EnableConfidentialCompute = true
		if t.instance.Scheduling == nil {
			t.instance.Scheduling = &compute.Scheduling{}
		}
		t.instance.Scheduling.OnHostMaintenance = "TERMINATE"
	} else if t.instancebeta != nil {
		if t.instancebeta.ConfidentialInstanceConfig == nil {
			t.instancebeta.ConfidentialInstanceConfig = &computeBeta.ConfidentialInstanceConfig{}
		}
		t.instancebeta.ConfidentialInstanceConfig.EnableConfidentialCompute = true
		if t.instancebeta.Scheduling == nil {
			t.instancebeta.Scheduling = &computeBeta.Scheduling{}
		}
		t.instancebeta.Scheduling.OnHostMaintenance = "TERMINATE"
	}
}

// SetMinCPUPlatform sets the minimum CPU platform of the instance.
func (t *TestVM) SetMinCPUPlatform(minCPUPlatform string) {
	if t.instance != nil {
		t.instance.MinCpuPlatform = minCPUPlatform
	} else if t.instancebeta != nil {
		t.instancebeta.MinCpuPlatform = minCPUPlatform
	}
}

// UseGVNIC sets the type of vNIC to be used to GVNIC
func (t *TestVM) UseGVNIC() {
	if t.instance != nil {
		if t.instance.NetworkInterfaces == nil {
			t.instance.NetworkInterfaces = []*compute.NetworkInterface{
				{
					NicType: "GVNIC",
				},
			}
		} else {
			t.instance.NetworkInterfaces[0].NicType = "GVNIC"
		}
	} else if t.instancebeta != nil {
		if t.instancebeta.NetworkInterfaces == nil {
			t.instancebeta.NetworkInterfaces = []*computeBeta.NetworkInterface{
				{
					NicType: "GVNIC",
				},
			}
		} else {
			t.instancebeta.NetworkInterfaces[0].NicType = "GVNIC"
		}
	}
}

// AddCustomNetwork add current test VMs in workflow using provided network and
// subnetwork. If subnetwork is empty, not using subnetwork, in this case
// network has to be in auto mode VPC.
func (t *TestVM) AddCustomNetwork(network *Network, subnetwork *Subnetwork) error {
	var subnetworkName string
	if subnetwork == nil {
		subnetworkName = ""
		if !*network.network.AutoCreateSubnetworks {
			return fmt.Errorf("network %s is not auto mode, subnet is required", network.name)
		}
	} else {
		subnetworkName = subnetwork.name
	}

	if t.instance != nil {
		// Add network config.
		networkInterface := compute.NetworkInterface{
			Network:    network.name,
			Subnetwork: subnetworkName,
			AccessConfigs: []*compute.AccessConfig{
				{
					Type: "ONE_TO_ONE_NAT",
				},
			},
		}
		if t.instance.NetworkInterfaces == nil {
			t.instance.NetworkInterfaces = []*compute.NetworkInterface{&networkInterface}
		} else {
			t.instance.NetworkInterfaces = append(t.instance.NetworkInterfaces, &networkInterface)
		}
	} else if t.instancebeta != nil {
		// Add network config.
		networkInterface := computeBeta.NetworkInterface{
			Network:    network.name,
			Subnetwork: subnetworkName,
			AccessConfigs: []*computeBeta.AccessConfig{
				{
					Type: "ONE_TO_ONE_NAT",
				},
			},
		}
		if t.instancebeta.NetworkInterfaces == nil {
			t.instancebeta.NetworkInterfaces = []*computeBeta.NetworkInterface{&networkInterface}
		} else {
			t.instancebeta.NetworkInterfaces = append(t.instancebeta.NetworkInterfaces, &networkInterface)
		}
	}

	return nil
}

// AddAliasIPRanges add alias ip range to current test VMs.
func (t *TestVM) AddAliasIPRanges(aliasIPRange, rangeName string) error {
	// TODO: If we haven't set any NetworkInterface struct, does it make sense to support adding alias IPs?
	if t.instance != nil {
		if t.instance.NetworkInterfaces == nil {
			return fmt.Errorf("must call AddCustomNetwork prior to AddAliasIPRanges")
		}
		t.instance.NetworkInterfaces[0].AliasIpRanges = append(t.instance.NetworkInterfaces[0].AliasIpRanges, &compute.AliasIpRange{
			IpCidrRange:         aliasIPRange,
			SubnetworkRangeName: rangeName,
		})
	} else if t.instancebeta != nil {
		if t.instancebeta.NetworkInterfaces == nil {
			return fmt.Errorf("must call AddCustomNetwork prior to AddAliasIPRanges")
		}
		t.instancebeta.NetworkInterfaces[0].AliasIpRanges = append(t.instancebeta.NetworkInterfaces[0].AliasIpRanges, &computeBeta.AliasIpRange{
			IpCidrRange:         aliasIPRange,
			SubnetworkRangeName: rangeName,
		})
	}
	return nil
}

// SetPrivateIP set IPv4 internal IP address for target network to the current test VMs.
func (t *TestVM) SetPrivateIP(network *Network, networkIP string) error {
	if t.instance != nil {
		if t.instance.NetworkInterfaces == nil {
			return fmt.Errorf("must call AddCustomNetwork prior to AddPrivateIP")
		}
		for _, nic := range t.instance.NetworkInterfaces {
			if nic.Network == network.name {
				nic.NetworkIP = networkIP
				return nil
			}
		}
	} else if t.instancebeta != nil {
		if t.instancebeta.NetworkInterfaces == nil {
			return fmt.Errorf("must call AddCustomNetwork prior to AddPrivateIP")
		}
		for _, nic := range t.instancebeta.NetworkInterfaces {
			if nic.Network == network.name {
				nic.NetworkIP = networkIP
				return nil
			}
		}
	}

	return fmt.Errorf("not found network interface %s", network.name)
}

// Network represent network used by vm in setup.go.
type Network struct {
	name         string
	testWorkflow *TestWorkflow
	network      *daisy.Network
}

// Subnetwork represent subnetwork used by vm in setup.go.
type Subnetwork struct {
	name         string
	testWorkflow *TestWorkflow
	subnetwork   *daisy.Subnetwork
	network      *Network
}

// CreateNetwork creates custom network. Using AddCustomNetwork method provided by
// TestVM to config network on vm
func (t *TestWorkflow) CreateNetwork(networkName string, autoCreateSubnetworks bool) (*Network, error) {
	createNetworkStep, network, err := t.appendCreateNetworkStep(networkName, DefaultMTU, autoCreateSubnetworks)
	if err != nil {
		return nil, err
	}

	createVMsStep, ok := t.wf.Steps[createVMsStepName]
	if ok {
		if err := t.wf.AddDependency(createVMsStep, createNetworkStep); err != nil {
			return nil, err
		}
	}

	return &Network{networkName, t, network}, nil
}

// SetMTU sets the MTU of the network. The MTU must be between 1460 and 8896, inclusively.
func (n *Network) SetMTU(mtu int) {
	if mtu >= DefaultMTU && mtu <= JumboFramesMTU {
		n.network.Mtu = int64(mtu)
	}
}

// CreateSubnetwork creates custom subnetwork. Using AddCustomNetwork method
// provided by TestVM to config network on vm
func (n *Network) CreateSubnetwork(name string, ipRange string) (*Subnetwork, error) {
	createSubnetworksStep, subnetwork, err := n.testWorkflow.appendCreateSubnetworksStep(name, ipRange, n.name)
	if err != nil {
		return nil, err
	}
	createNetworkStep, ok := n.testWorkflow.wf.Steps[createNetworkStepName]
	if !ok {
		return nil, fmt.Errorf("create-network step missing")
	}
	if err := n.testWorkflow.wf.AddDependency(createSubnetworksStep, createNetworkStep); err != nil {
		return nil, err
	}

	return &Subnetwork{name, n.testWorkflow, subnetwork, n}, nil
}

// SetRegion sets the subnetwork region
func (s *Subnetwork) SetRegion(region string) {
	s.subnetwork.Region = region
}

// SetPurpose sets the subnetwork purpose
func (s *Subnetwork) SetPurpose(purpose string) {
	s.subnetwork.Purpose = purpose
}

// SetRole sets the subnetwork role
func (s *Subnetwork) SetRole(role string) {
	s.subnetwork.Role = role
}

// AddSecondaryRange add secondary IP range to Subnetwork
func (s Subnetwork) AddSecondaryRange(rangeName, ipRange string) {
	s.subnetwork.SecondaryIpRanges = append(s.subnetwork.SecondaryIpRanges, &compute.SubnetworkSecondaryRange{
		IpCidrRange: ipRange,
		RangeName:   rangeName,
	})
}

func (t *TestWorkflow) appendCreateFirewallStep(firewallName, networkName, protocol string, ports, ranges []string) (*daisy.Step, *daisy.FirewallRule, error) {
	if ranges == nil {
		ranges = []string{DefaultSourceRange}
	}
	firewall := &daisy.FirewallRule{
		Firewall: compute.Firewall{
			Name:         firewallName,
			Network:      networkName,
			SourceRanges: ranges,
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: protocol,
					Ports:      ports,
				},
			},
		},
	}

	createFirewallRules := &daisy.CreateFirewallRules{}
	*createFirewallRules = append(*createFirewallRules, firewall)
	createFirewallStep, ok := t.wf.Steps[createFirewallStepName]
	if ok {
		// append to existing step.
		*createFirewallStep.CreateFirewallRules = append(*createFirewallStep.CreateFirewallRules, firewall)
	} else {
		var err error
		createFirewallStep, err = t.wf.NewStep(createFirewallStepName)
		if err != nil {
			return nil, nil, err
		}
		createFirewallStep.CreateFirewallRules = createFirewallRules
	}

	return createFirewallStep, firewall, nil
}

// AddSSHKey generate ssh key pair and return public key.
func (t *TestWorkflow) AddSSHKey(user string) (string, error) {
	keyFileName := os.TempDir() + "/id_rsa_" + uuid.New().String()
	if _, err := os.Stat(keyFileName); os.IsExist(err) {
		os.Remove(keyFileName)
	}
	commandArgs := []string{"-t", "rsa", "-f", keyFileName, "-N", "", "-q"}
	cmd := exec.Command("ssh-keygen", commandArgs...)
	if out, err := cmd.Output(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("ssh-keygen failed: %s %s %v", out, exitErr.Stderr, err)
		}
		return "", fmt.Errorf("ssh-keygen failed: %v %v", out, err)
	}

	publicKey, err := ioutil.ReadFile(keyFileName + ".pub")
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %v", err)
	}
	sourcePath := fmt.Sprintf("%s-ssh-key", user)
	t.wf.Sources[sourcePath] = keyFileName

	return string(publicKey), nil
}

// CreateFirewallRule create firewall rule.
func (n *Network) CreateFirewallRule(firewallName, protocol string, ports, ranges []string) error {
	createFirewallStep, _, err := n.testWorkflow.appendCreateFirewallStep(firewallName, n.name, protocol, ports, ranges)
	if err != nil {
		return err
	}

	createNetworkStep, ok := n.testWorkflow.wf.Steps[createNetworkStepName]
	if ok {
		if err := n.testWorkflow.wf.AddDependency(createFirewallStep, createNetworkStep); err != nil {
			return err
		}
	}
	createVMsStep, ok := n.testWorkflow.wf.Steps[createVMsStepName]
	if ok {
		if err := n.testWorkflow.wf.AddDependency(createVMsStep, createFirewallStep); err != nil {
			return err
		}
	}

	return nil
}
