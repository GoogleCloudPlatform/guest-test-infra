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
	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

// Name is the name of the test package. It must match the directory name.
const Name = "guestagent"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	telemetrydisabled, err := t.CreateTestVM("telemetry-disabled")
	if err != nil {
		return err
	}
	telemetrydisabled.AddMetadata("disable-guest-telemetry", "true")
	telemetrydisabled.RunTests("TestTelemetryDisabled")
	telemetryenabled, err := t.CreateTestVM("telemetry-enabled")
	if err != nil {
		return err
	}
	telemetryenabled.RunTests("TestTelemetryEnabled") // Enabled by default
	if utils.HasFeature(t.Image, "WINDOWS") {
		passwordInst := &daisy.Instance{}
		passwordInst.Scopes = append(passwordInst.Scopes, "https://www.googleapis.com/auth/cloud-platform")
		windowsaccountVM, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: "windowsaccount"}}, passwordInst)
		if err != nil {
			return err
		}
		windowsaccountVM.RunTests("TestWindowsPasswordReset")
	}
	return nil
}
