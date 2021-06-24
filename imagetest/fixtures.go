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

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"google.golang.org/api/compute/v1"
)

const (
	createVMsStepName        = "create-vms"
	createDisksStepName      = "create-disks"
	createNetworkStepName    = "create-network"
	createSubNetworkStepName = "create-sub-network"
	successMatch             = "FINISHED-TEST"
)

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

	waitStopStep, err := t.testWorkflow.addWaitStep("stopped-"+stepSuffix, t.name, true)
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

	waitStartedStep, err := t.testWorkflow.addWaitStep("started-"+stepSuffix, t.name, false)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(waitStartedStep, startInstancesStep); err != nil {
		return err
	}
	return nil
}

// ResizeDiskAndReboot resize the disk of the current test VMs and reboot
func (t *TestVM) ResizeDiskAndReboot(vmname string, diskSize int) error {
	t.testWorkflow.counter++
	stepSuffix := fmt.Sprintf("%s-%d", t.name, t.testWorkflow.counter)

	lastStep, err := t.testWorkflow.getLastStepForVM(vmname)
	if err != nil {
		return fmt.Errorf("failed resolve last step")
	}

	diskResizeStep, err := t.testWorkflow.addDiskResizeStep(stepSuffix, vmname, diskSize)
	if err != nil {
		return err
	}

	if err := t.testWorkflow.wf.AddDependency(diskResizeStep, lastStep); err != nil {
		return err
	}

	return t.Reboot()
}

// EnableSecureBoot make the current test VMs in workflow with secure boot.
func (t *TestVM) EnableSecureBoot() {
	for _, i := range t.testWorkflow.wf.Steps[createVMsStepName].CreateInstances.Instances {
		if i.Name == t.name {
			i.ShieldedInstanceConfig = &compute.ShieldedInstanceConfig{
				EnableSecureBoot: true,
			}
			break
		}
	}
}

// SetCustomNetwork set current test VMs in workflow using provided network and
// subnetwork.
func (t *TestVM) SetCustomNetwork(networkName, subnetworkName string) error {
	for _, i := range t.testWorkflow.wf.Steps[createVMsStepName].CreateInstances.Instances {
		if i.Name == t.name {
			networkInterface := compute.NetworkInterface{
				Network:    networkName,
				Subnetwork: subnetworkName,
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
					},
				},
			}

			i.NetworkInterfaces = []*compute.NetworkInterface{&networkInterface}
			break
		}
	}
	return nil
}

// AddAliasIPRanges add alias ip range to current test VMs.
func (t *TestVM) AddAliasIPRanges(aliasIPRange, rangeName string) error {
	for _, i := range t.testWorkflow.wf.Steps[createVMsStepName].CreateInstances.Instances {
		if i.Name == t.name {
			i.NetworkInterfaces[0].AliasIpRanges = append(i.NetworkInterfaces[0].AliasIpRanges, &compute.AliasIpRange{
				IpCidrRange:         aliasIPRange,
				SubnetworkRangeName: rangeName,
			})
			break
		}
	}
	return nil
}
