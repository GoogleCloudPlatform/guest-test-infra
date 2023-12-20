// Copyright 2023 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     https://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package guestagent

import (
	"github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
const Name = "guestagent"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {

	telemetrydisabledinst := &daisy.Instance{}
	telemetrydisabledinst.Scopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
	telemetrydisabledinst.Name = "telemetry-disabled"
	telemetrydisabledvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: telemetrydisabledinst.Name, Type: imagetest.PdBalanced}}, telemetrydisabledinst)
	if err != nil {
		return err
	}
	telemetrydisabledvm.AddMetadata("disable-guest-telemetry", "true")
	telemetrydisabledvm.RunTests("TestTelemetryDisabled")

	telemetryenabledinst := &daisy.Instance{}
	telemetryenabledinst.Scopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
	telemetryenabledinst.Name = "telemetry-enabled"
	telemetryenabledvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: telemetryenabledinst.Name, Type: imagetest.PdBalanced}}, telemetryenabledinst)
	if err != nil {
		return err
	}
	telemetryenabledvm.AddMetadata("disable-guest-telemetry", "false")
	telemetryenabledvm.RunTests("TestTelemetryEnabled")

	if !utils.HasFeature(t.Image, "WINDOWS") {
		snapshotinst := &daisy.Instance{}
		snapshotinst.Scopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
		snapshotinst.Name = "snapshot-scripts"
		snapshotvm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: snapshotinst.Name, Type: imagetest.PdBalanced}}, snapshotinst)
		if err != nil {
			return err
		}
		snapshotvm.RunTests("TestSnapshotScripts")
	}
	return nil
}
