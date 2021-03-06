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
	"google.golang.org/api/compute/v1"
)

const (
	createVMsStepName        = "create-vms"
	createDisksStepName      = "create-disks"
	createNetworkStepName    = "create-networks"
	createSubnetworkStepName = "create-sub-networks"
	successMatch             = "FINISHED-TEST"
)

// TestVM is a test VM.
type TestVM struct {
	name         string
	testWorkflow *TestWorkflow
	instance     *daisy.Instance
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
	parts := strings.Split(name, ".")
	vmname := strings.ReplaceAll(parts[0], "_", "-")

	createDisksStep, err := t.appendCreateDisksStep(vmname)
	if err != nil {
		return nil, err
	}

	// createDisksStep doesn't depend on any other steps.
	createVMStep, i, err := t.appendCreateVMStep(vmname, name)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return nil, err
	}

	waitStep, err := t.addWaitStep(vmname, vmname, false)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	if createNetworksStep, ok := t.wf.Steps[createNetworkStepName]; ok {
		if err := t.wf.AddDependency(createVMStep, createNetworksStep); err != nil {
			return nil, err
		}
	}

	return &TestVM{name: vmname, testWorkflow: t, instance: i}, nil
}

// AddMetadata adds the specified key:value pair to metadata during VM creation.
func (t *TestVM) AddMetadata(key, value string) {
	if t.instance.Metadata == nil {
		t.instance.Metadata = make(map[string]string)
	}
	t.instance.Metadata[key] = value

	return
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

// EnableSecureBoot make the current test VMs in workflow with secure boot.
func (t *TestVM) EnableSecureBoot() {
	t.instance.ShieldedInstanceConfig = &compute.ShieldedInstanceConfig{
		EnableSecureBoot: true,
	}
}

// SetCustomNetwork set current test VMs in workflow using provided network and
// subnetwork. If subnetwork is empty, not using subnetwork, in this case
// network has to be in auto mode VPC.
func (t *TestVM) SetCustomNetwork(network *Network, subnetwork *Subnetwork) error {
	var subnetworkName string
	if subnetwork == nil {
		subnetworkName = ""
		if !*network.network.AutoCreateSubnetworks {
			return fmt.Errorf("network %s is not auto mode, subnet is required", network.name)
		}
	} else {
		subnetworkName = subnetwork.name
	}

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
	t.instance.NetworkInterfaces = []*compute.NetworkInterface{&networkInterface}

	return nil
}

// AddAliasIPRanges add alias ip range to current test VMs.
func (t *TestVM) AddAliasIPRanges(aliasIPRange, rangeName string) error {
	// TODO: If we haven't set any NetworkInterface struct, does it make sense to support adding alias IPs?
	if t.instance.NetworkInterfaces == nil {
		return fmt.Errorf("Must call SetCustomNetwork prior to AddAliasIPRanges")
	}
	t.instance.NetworkInterfaces[0].AliasIpRanges = append(t.instance.NetworkInterfaces[0].AliasIpRanges, &compute.AliasIpRange{
		IpCidrRange:         aliasIPRange,
		SubnetworkRangeName: rangeName,
	})

	return nil
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

// CreateNetwork creates custom network. Using SetCustomNetwork method provided by
// TestVM to config network on vm
func (t *TestWorkflow) CreateNetwork(networkName string, autoCreateSubnetworks bool) (*Network, error) {
	createNetworkStep, network, err := t.appendCreateNetworkStep(networkName, autoCreateSubnetworks)
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

// CreateSubnetwork creates custom subnetwork. Using SetCustomNetwork method
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

// AddSecondaryRange add secondary IP range to Subnetwork
func (s Subnetwork) AddSecondaryRange(rangeName, ipRange string) {
	s.subnetwork.SecondaryIpRanges = append(s.subnetwork.SecondaryIpRanges, &compute.SubnetworkSecondaryRange{
		IpCidrRange: ipRange,
		RangeName:   rangeName,
	})
}
