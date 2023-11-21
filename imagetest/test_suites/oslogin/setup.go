// Copyright 2023 Google LLC
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

package oslogin

import (
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "oslogin"

const (
	computeScope  = "https://www.googleapis.com/auth/compute"
	platformScope = "https://www.googleapis.com/auth/cloud-platform"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if utils.HasFeature(t.Image, "WINDOWS") {
		t.Skip("OSLogin not supported on windows")
		return nil
	}

	defaultVM, err := t.CreateTestVM("default")
	if err != nil {
		return err
	}
	defaultVM.AddScope(computeScope)
	defaultVM.AddMetadata("enable-oslogin", "true")
	defaultVM.RunTests("TestOsLoginEnabled|TestGetentPasswd|TestAgent")

	ssh, err := t.CreateTestVM("ssh")
	if err != nil {
		return err
	}
	ssh.AddScope(platformScope)
	ssh.AddMetadata("enable-oslogin", "false")
	ssh.RunTests("TestOsLoginDisabled|TestSSH|TestAdminSSH|Test2FASSH|Test2FAAdminSSH")

	twofa, err := t.CreateTestVM("twofa")
	if err != nil {
		return err
	}
	twofa.AddScope(computeScope)
	twofa.AddMetadata("enable-oslogin", "true")
	twofa.AddMetadata("enable-oslogin-2fa", "true")
	twofa.RunTests("TestAgent")
	return nil
}
