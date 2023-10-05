package imagetest

import (
	"testing"

	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"google.golang.org/api/compute/v1"
)

// TestAddMetadata tests that *TestVM.AddMetadata succeeds and that it
// populates the instance.Metadata map.
func TestAddMetadata(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	tvm.AddMetadata("key", "val")
	if tvm.instance.Metadata == nil {
		t.Errorf("failed to set VM metadata")
	}
	if val, ok := tvm.instance.Metadata["key"]; !ok || val != "val" {
		t.Errorf("invalid metadata set")
	}
	tvm.AddMetadata("key", "val2")
	if val, ok := tvm.instance.Metadata["key"]; !ok || val != "val2" {
		t.Errorf("invalid metadata set")
	}
}

// TestReboot tests that *TestVM.Reboot succeeds and that the appropriate stop
// and new final wait steps are created in the workflow.
func TestReboot(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if twf.counter != 0 {
		t.Errorf("step counter not starting at 0")
	}
	if err := tvm.Reboot(); err != nil {
		t.Errorf("failed to reboot: %v", err)
	}
	if twf.counter != 1 {
		t.Errorf("step counter not incremented")
	}
	if _, ok := twf.wf.Steps["stop-vm-1"]; !ok {
		t.Errorf("wait-vm-1 step missing")
	}
	lastStep, err := twf.getLastStepForVM("vm")
	if err != nil {
		t.Errorf("failed to get last step for vm: %v", err)
	}
	if lastStep.WaitForInstancesSignal == nil {
		t.Error("not wait step")
	}
	if step, ok := twf.wf.Steps["wait-started-vm-1"]; !ok || step != lastStep {
		t.Error("not wait-started-vm-1 step")
	}
}

// TestCreateVMMultipleDisks tests that after creating a VM with multiple disks,
// the correct step dependencies are in place.
func TestCreateVMMultipleDisks(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	disks := []*compute.Disk{{Name: "vm"}, {Name: "mountdisk", Type: PdSsd, SizeGb: 100}}
	tvm, err := twf.CreateTestVMMultipleDisks(disks, map[string]string{})
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	// once found, expect createInstancesStep.CreateInstances != nil
	// once found, expect createDisksStep.CreateDisks != nil
	var createInstancesStep, createDisksStep *daisy.Step
	for _, step := range twf.wf.Steps {
		// there should only be one create instance step
		if step.CreateInstances != nil {
			if createInstancesStep == nil {
				createInstancesStep = step
			} else {
				t.Errorf("workflow has multiple create instance steps when it should not")
			}
		}

		if step.CreateDisks != nil {
			if createDisksStep == nil {
				createDisksStep = step
			} else {
				t.Errorf("workflow has multiple create disk steps when it should not")
			}
		}
	}

	if createInstancesStep == nil || createInstancesStep.CreateInstances == nil {
		t.Errorf("failed to find create instances step when creating multiple disks")
	}

	if createDisksStep == nil || createDisksStep.CreateDisks == nil {
		t.Errorf("failed to find create disks step when creating multiple disks")
	}

	daisyStepDisksSlice := *(createDisksStep.CreateDisks)
	if len(disks) != len(daisyStepDisksSlice) {
		t.Errorf("found incorrect number of disks in create disk step: expected %d, got %d",
			len(disks), len(daisyStepDisksSlice))
	}

	if twf.counter != 0 {
		t.Errorf("step counter not starting at 0")
	}
	if err := tvm.Reboot(); err != nil {
		t.Errorf("failed to reboot: %v", err)
	}
	if twf.counter != 1 {
		t.Errorf("step counter not incremented")
	}
	if _, ok := twf.wf.Steps["stop-vm-1"]; !ok {
		t.Errorf("wait-vm-1 step missing")
	}
	lastStep, err := twf.getLastStepForVM("vm")
	if err != nil {
		t.Errorf("failed to get last step for vm: %v", err)
	}
	if lastStep.WaitForInstancesSignal == nil {
		t.Error("not wait step")
	}
	if step, ok := twf.wf.Steps["wait-started-vm-1"]; !ok || step != lastStep {
		t.Error("not wait-started-vm-1 step")
	}
}

// TestRebootMultipleDisks creates a VM using multiple disks, and then runs
// the same tests as TestReboot.
func TestRebootMultipleDisks(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	disks := []*compute.Disk{{Name: "vm"}, {Name: "mountdisk", Type: PdBalanced, SizeGb: 100}}
	testMachineType := "c3-standard-4"
	pdBalancedParams := map[string]string{"machineType": testMachineType}
	tvm, err := twf.CreateTestVMMultipleDisks(disks, pdBalancedParams)
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if tvm.instance.MachineType != testMachineType {
		t.Errorf("failed to set test machine type, expected %s but got %s", testMachineType, tvm.instance.MachineType)
	}
	if twf.counter != 0 {
		t.Errorf("step counter not starting at 0")
	}
	if err := tvm.Reboot(); err != nil {
		t.Errorf("failed to reboot: %v", err)
	}
	if twf.counter != 1 {
		t.Errorf("step counter not incremented")
	}
	if _, ok := twf.wf.Steps["stop-vm-1"]; !ok {
		t.Errorf("wait-vm-1 step missing")
	}
	lastStep, err := twf.getLastStepForVM("vm")
	if err != nil {
		t.Errorf("failed to get last step for vm: %v", err)
	}
	if lastStep.WaitForInstancesSignal == nil {
		t.Error("not wait step")
	}
	if step, ok := twf.wf.Steps["wait-started-vm-1"]; !ok || step != lastStep {
		t.Error("not wait-started-vm-1 step")
	}
}

// TestResizeDiskAndReboot tests that *TestVM.ResizeDiskAndReboot succeeds and
// that the appropriate resize and new final wait steps are created in the
// workflow.
func TestResizeDiskAndReboot(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if err := tvm.ResizeDiskAndReboot(200); err != nil {
		t.Errorf("failed to reboot: %v", err)
	}
	if _, ok := twf.wf.Steps["resize-disk-vm-1"]; !ok {
		t.Errorf("wait-vm-1 step missing")
	}
	step, err := twf.getLastStepForVM("vm")
	if err != nil {
		t.Errorf("failed to get last step for vm: %v", err)
	}
	if step.WaitForInstancesSignal == nil {
		t.Error("not wait step")
	}
	if twf.wf.Steps["wait-started-vm-2"] != step {
		t.Error("not wait-started-vm-2 step")
	}
}

// TestEnableSecureBoot tests that *TestVM.EnableSecureBoot succeeds and
// populates the ShieldedInstanceConfig struct.
func TestEnableSecureBoot(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if tvm.instance.ShieldedInstanceConfig != nil {
		t.Errorf("VM didn't have nil SIC at creation")
	}
	tvm.EnableSecureBoot()
	if tvm.instance.ShieldedInstanceConfig == nil {
		t.Errorf("VM SIC is nil")
	}
}

// TestWaitForVMQuota tests that WaitForVMQuota successfully appends a quota to
// the wait for vm quota step.
func TestWaitForVMQuota(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	quota := &daisy.QuotaAvailable{Metric: "test", Units: 1, Region: "us-central1"}
	err = twf.WaitForVMQuota(quota)
	if err != nil {
		t.Errorf("failed to append quota: %v", err)
	}
	quotaStep, ok := twf.wf.Steps[waitForVMQuotaStepName]
	if !ok {
		t.Errorf("Could not find wait for vm quota step")
	}
	if quotaStep.WaitForAvailableQuotas.Quotas[0] != quota {
		t.Errorf("An unexpected quota was appended")
	}
}

// TestUseGVNIC tests that *TestVM.UseGVNIC succeeds and
// populates the Network Interface with a NIC type of GVNIC.
func TestUseGVNIC(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	tvm.UseGVNIC()
	if tvm.instance.NetworkInterfaces == nil {
		t.Errorf("VM Network Interfaces is nil")
	}
	if tvm.instance.NetworkInterfaces[0].NicType != "GVNIC" {
		t.Errorf("VM Network Interface type not set to GVNIC")
	}
}

// TestAddAliasIPRanges tests that *TestVM.AddAliasIPRanges succeeds and that
// it fails if *TestVM.AddCustomNetwork hasn't been called first.
func TestAddAliasIPRanges(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if err := tvm.AddAliasIPRanges("aliasIPRange", "rangeName"); err == nil {
		t.Fatalf("shouldn't be able to set alias IP without calling setcustomnetwork")
	}
	network, err := twf.CreateNetwork("network", true)
	if err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if err := tvm.AddCustomNetwork(network, nil); err != nil {
		t.Errorf("failed to set custom network: %v", err)
	}
	if err := tvm.AddAliasIPRanges("aliasIPRange", "rangeName"); err != nil {
		t.Fatalf("error adding alias IP range: %v", err)
	}
	if tvm.instance.NetworkInterfaces[0].AliasIpRanges == nil {
		t.Errorf("VM alias IP is nil")
	}
}

// TestSetCustomNetwork tests that *TestVM.AddCustomNetwork succeeds and that
// it fails if testworkflow.CreateNetwork has not been called first.
func TestSetCustomNetwork(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	network, err := twf.CreateNetwork("network", true)
	if err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if err := tvm.AddCustomNetwork(network, nil); err != nil {
		t.Errorf("failed to set custom network: %v", err)
	}
}

// TestSetCustomNetworkAndSubnetwork tests that *TestVM.AddCustomNetwork
// succeeds with a subnet argument and that it fails if
// *Network.CreateSubnetwork has not been called first.
func TestSetCustomNetworkAndSubnetwork(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	network, err := twf.CreateNetwork("network", false)
	if err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if err := tvm.AddCustomNetwork(network, nil); err == nil {
		t.Errorf("should have gotten an error using no subnet with custom mode network.")
	}
	subnet, err := network.CreateSubnetwork("subnet", "ipRange")
	if err != nil {
		t.Errorf("failed to create subnetwork: %v", err)
	}
	if err := tvm.AddCustomNetwork(network, subnet); err != nil {
		t.Errorf("failed to set custom network and subnetwork: %v", err)
	}
}

// TestAddSecondaryRange tests that AddSecondaryRange populates the
// subnet.SecondaryIpRanges struct.
func TestAddSecondaryRange(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	network, err := twf.CreateNetwork("network", false)
	if err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	subnet, err := network.CreateSubnetwork("subnet", "ipRange")
	if err != nil {
		t.Errorf("failed to create subnetwork: %v", err)
	}
	if subnet.subnetwork.SecondaryIpRanges != nil {
		t.Errorf("Subnet didn't have nil secondary ranges at creation")
	}
	subnet.AddSecondaryRange("rangeName", "ipRange")
	if subnet.subnetwork.SecondaryIpRanges == nil {
		t.Errorf("Subnet has nil secondary range")
	}
}

// TestCreateNetworkDependenciesReverse tests that the create-vms step depends
// on the create-networks step if they are created in order.
func TestCreateNetworkDependencies(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if _, err = twf.CreateNetwork("network", false); err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if _, err = twf.CreateTestVM("vm"); err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if _, ok := twf.wf.Dependencies[createNetworkStepName]; ok {
		t.Errorf("network step has unnecessary dependencies")
	}
	deps, ok := twf.wf.Dependencies[createVMsStepName]
	if !ok {
		t.Errorf("create-vms step missing dependencies")
	}
	var found bool
	for _, dep := range deps {
		if dep == createNetworkStepName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("create-vms step does not depend on create-networks step")
	}
}

// TestCreateNetworkDependenciesReverse tests that the create-vms step depends
// on the create-networks step if they are created in reverse.
func TestCreateNetworkDependenciesReverse(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if _, err = twf.CreateTestVM("vm"); err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if _, err = twf.CreateNetwork("network", false); err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	if _, ok := twf.wf.Dependencies[createNetworkStepName]; ok {
		t.Errorf("network step has unnecessary dependencies")
	}
	deps, ok := twf.wf.Dependencies[createVMsStepName]
	if !ok {
		t.Errorf("create-vms step missing dependencies")
	}
	var found bool
	for _, dep := range deps {
		if dep == createNetworkStepName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("create-vms step does not depend on create-networks step")
	}
}

func TestAddUser(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create network: %v", err)
	}
	tvm.AddUser("username", "PUBKEY1")
	if tvm.instance.Metadata == nil {
		t.Fatalf("instance metadata is nil")
	}
	keys, ok := tvm.instance.Metadata["ssh-keys"]
	if !ok {
		t.Fatalf("\"ssh-keys\" key not added to instance")
	}
	if keys != "username:PUBKEY1" {
		t.Fatalf("\"ssh-keys\" key malformed")
	}
	tvm.AddUser("username2", "PUBKEY2")
	if keys, ok := tvm.instance.Metadata["ssh-keys"]; !ok || keys != "username:PUBKEY1\nusername2:PUBKEY2" {
		t.Errorf("\"ssh-keys\" key malformed after repeated entry")
	}
}

func TestForceMachineType(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if tvm.instance.MachineType != "" {
		t.Errorf("machine type already set: %v", tvm.instance.MachineType)
	}
	tvm.ForceMachineType("t2a-standard-1")
	if tvm.instance.MachineType != "t2a-standard-1" {
		t.Errorf("could not set test machine type, got %q, want t2a-standard-1", tvm.instance.MachineType)
	}
}

func TestForceZone(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image", "30m", "x86", "arm64")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	tvm, err := twf.CreateTestVM("vm")
	if err != nil {
		t.Errorf("failed to create test vm: %v", err)
	}
	if tvm.instance.Zone != "" {
		t.Errorf("machine zone already set: %v", tvm.instance.Zone)
	}
	tvm.ForceZone("us-east1-a")
	if tvm.instance.Zone != "us-east1-a" {
		t.Errorf("could not set test zone, got %q, want us-east1-a", tvm.instance.Zone)
	}
}
