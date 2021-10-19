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

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"github.com/google/uuid"
	"google.golang.org/api/compute/v1"
)

const (
	createVMsStepName        = "create-vms"
	createDisksStepName      = "create-disks"
	createNetworkStepName    = "create-networks"
	createFirewallStepName   = "create-firewalls"
	createSubnetworkStepName = "create-sub-networks"
	successMatch             = "FINISHED-TEST"
	DefaultSourceRange       = "10.128.0.0/9"
)

// TestVM is a test VM.
type TestVM struct {
	name         string
	testWorkflow *TestWorkflow
	instance     *daisy.Instance
}

// AddUser add user public key to metadata ssh-keys.
func (t *TestVM) AddUser(user, publicKey string) {
	keyline := fmt.Sprintf("%s:%s", user, publicKey)
	if keys, ok := t.instance.Metadata["ssh-keys"]; ok {
		keyline = fmt.Sprintf("%s\n%s", keys, keyline)
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

// CreateTestVM adds the necessary steps to create a VM with the specified
// name to the workflow.
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

// SetShutdownScriptURL sets the`shutdown-script-url` metadata key for a VM.
func (t *TestVM) SetShutdownScriptURL(script string) error {
	fileName := fmt.Sprintf("/shutdown_script-%s", uuid.New())
	if err := ioutil.WriteFile(fileName, []byte(script), 755); err != nil {
		return err
	}
	t.testWorkflow.wf.Sources["shutdown-script"] = fileName

	t.AddMetadata("shutdown-script-url", "${SOURCESPATH}/shutdown-script")
	return nil
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

	return nil
}

// AddAliasIPRanges add alias ip range to current test VMs.
func (t *TestVM) AddAliasIPRanges(aliasIPRange, rangeName string) error {
	// TODO: If we haven't set any NetworkInterface struct, does it make sense to support adding alias IPs?
	if t.instance.NetworkInterfaces == nil {
		return fmt.Errorf("Must call AddCustomNetwork prior to AddAliasIPRanges")
	}
	t.instance.NetworkInterfaces[0].AliasIpRanges = append(t.instance.NetworkInterfaces[0].AliasIpRanges, &compute.AliasIpRange{
		IpCidrRange:         aliasIPRange,
		SubnetworkRangeName: rangeName,
	})

	return nil
}

// SetPrivateIP set IPv4 internal IP address for target network to the current test VMs.
func (t *TestVM) SetPrivateIP(network *Network, networkIP string) error {
	if t.instance.NetworkInterfaces == nil {
		return fmt.Errorf("Must call AddCustomNetwork prior to AddPrivateIP")
	}
	for _, nic := range t.instance.NetworkInterfaces {
		if nic.Network == network.name {
			nic.NetworkIP = networkIP
			return nil
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
	keyFileName := "/id_rsa_" + uuid.New().String()
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
