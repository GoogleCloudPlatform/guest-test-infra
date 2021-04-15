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
	"testing"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
)

func TestAddStartStep(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	step, err := twf.addStartStep("stepname", "vmname")
	if err != nil {
		t.Errorf("failed to add start step to test workflow: %v", err)
	}
	if step.StartInstances == nil {
		t.Error("StartInstances step is missing")
	}
	if len(step.StartInstances.Instances) != 1 {
		t.Error("StartInstances step is malformed")
	}
	if step.StartInstances.Instances[0] != "vmname" {
		t.Error("StartInstances step is malformed")
	}
	stepFromWF, ok := twf.wf.Steps["start-stepname"]
	if !ok || step != stepFromWF {
		t.Error("Step was not correctly added to workflow")
	}
}

func TestAddStopStep(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	step, err := twf.addStopStep("stepname", "vmname")
	if err != nil {
		t.Errorf("failed to add stop step to test workflow: %v", err)
	}
	if step.StopInstances == nil {
		t.Error("StopInstances step is missing")
	}
	if len(step.StopInstances.Instances) != 1 {
		t.Error("StopInstances step is malformed")
	}
	if step.StopInstances.Instances[0] != "vmname" {
		t.Error("StopInstances step is malformed")
	}
	stepFromWF, ok := twf.wf.Steps["stop-stepname"]
	if !ok || step != stepFromWF {
		t.Error("step was not correctly added to workflow")
	}
}

func TestAddWaitStep(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	step, err := twf.addWaitStep("stepname", "vmname", false)
	if err != nil {
		t.Errorf("failed to add wait step to test workflow: %v", err)
	}
	if step.WaitForInstancesSignal == nil {
		t.Error("WaitForInstancesSignal step is missing")
	}
	instancesSignal := []*daisy.InstanceSignal(*step.WaitForInstancesSignal)
	if len(instancesSignal) != 1 {
		t.Error("waitInstances step is malformed")
	}
	if instancesSignal[0].Name != "vmname" {
		t.Error("waitInstances step is malformed")
	}
	if instancesSignal[0].SerialOutput.SuccessMatch != successMatch {
		t.Error("waitInstances step is malformed")
	}
	if instancesSignal[0].Stopped {
		t.Error("waitInstances step is malformed")
	}
	stepFromWF, ok := twf.wf.Steps["wait-stepname"]
	if !ok || step != stepFromWF {
		t.Error("step was not correctly added to workflow")
	}
}

func TestAddWaitStoppedStep(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	step, err := twf.addWaitStep("stepname", "vmname", true)
	if err != nil {
		t.Errorf("failed to add wait step to test workflow: %v", err)
	}
	if step.WaitForInstancesSignal == nil {
		t.Error("WaitForInstancesSignal step is missing")
	}
	instancesSignal := []*daisy.InstanceSignal(*step.WaitForInstancesSignal)
	if len(instancesSignal) != 1 {
		t.Error("waitInstances step is malformed")
	}
	if instancesSignal[0].Name != "vmname" {
		t.Error("waitInstances step is malformed")
	}
	if instancesSignal[0].SerialOutput != nil {
		t.Error("waitInstances step is malformed")
	}
	if !instancesSignal[0].Stopped {
		t.Error("waitInstances step is malformed")
	}
	stepFromWF, ok := twf.wf.Steps["wait-stepname"]
	if !ok || step != stepFromWF {
		t.Error("step was not correctly added to workflow")
	}
}

func TestAppendCreateDisksStep(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	step, err := twf.appendCreateDisksStep("diskname")
	if err != nil {
		t.Errorf("failed to add wait step to test workflow: %v", err)
	}
	if step.CreateDisks == nil {
		t.Error("CreateDisks step is missing")
	}
	disks := []*daisy.Disk(*step.CreateDisks)
	if len(disks) != 1 {
		t.Error("CreateDisks step is malformed")
	}
	if disks[0].Name != "diskname" {
		t.Error("CreateDisks step is malformed")
	}
	if disks[0].SourceImage != "image" {
		t.Error("CreateDisks step is malformed")
	}
	stepFromWF, ok := twf.wf.Steps["create-disks"]
	if !ok || step != stepFromWF {
		t.Error("step was not correctly added to workflow")
	}
	step2, err := twf.appendCreateDisksStep("diskname2")
	if err != nil {
		t.Fatalf("failed to add wait step to test workflow: %v", err)
	}
	if step2 != stepFromWF {
		t.Fatal("CreateDisks step was not appended")
	}
	disks = []*daisy.Disk(*step2.CreateDisks)
	if len(disks) != 2 {
		t.Fatal("CreateDisks step was not appended")
	}
	if disks[1].Name != "diskname2" {
		t.Error("CreateDisks step is malformed")
	}
}

func TestAppendCreateVMStep(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	if _, ok := twf.wf.Steps["create-disks"]; ok {
		t.Fatal("create-disks step already exists")
	}
	step, err := twf.appendCreateVMStep("vmname")
	if err != nil {
		t.Errorf("failed to add wait step to test workflow: %v", err)
	}
	if step.CreateInstances == nil {
		t.Error("CreateDisks step is missing")
	}
	instances := step.CreateInstances.Instances
	if len(instances) != 1 {
		t.Error("CreateInstances step is malformed")
	}
	if instances[0].Name != "vmname" {
		t.Error("CreateInstances step is malformed")
	}
	stepFromWF, ok := twf.wf.Steps["create-vms"]
	if !ok || step != stepFromWF {
		t.Error("step was not correctly added to workflow")
	}
	step2, err := twf.appendCreateVMStep("vmname2")
	if err != nil {
		t.Fatalf("failed to add wait step to test workflow: %v", err)
	}
	if step2 != stepFromWF {
		t.Fatal("CreateDisks step was not appended")
	}
	instances = step.CreateInstances.Instances
	if len(instances) != 2 {
		t.Fatal("CreateDisks step was not appended")
	}
	if instances[1].Name != "vmname2" {
		t.Error("CreateInstances step is malformed")
	}
}

func TestNewTestWorkflow(t *testing.T) {
	twf, err := NewTestWorkflow("name", "image")
	if err != nil {
		t.Errorf("failed to create test workflow: %v", err)
	}
	if twf.wf == nil {
		t.Error("test workflow is malformed")
	}
	if len(twf.wf.Steps) != 0 {
		t.Error("test workflow has initial steps")
	}
}
