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

// test2FAUser encapsulates a test user for 2FA tests.
type test2FAUser struct {
	// email is the secret for the email of this test user.
	email string

	// twoFA is the secret for the 2FA secret of this test user.
	twoFA string

	// sshKey is the secret for the private SSH key of this test user.
	sshKey string
}

const (
	computeScope  = "https://www.googleapis.com/auth/compute"
	platformScope = "https://www.googleapis.com/auth/cloud-platform"

	// 2FA metadata keys.
	normal2FAUser   = "normal-2fa-user"
	normal2FAKey    = "normal-2fa-key"
	normal2FASSHKey = "normal-2fa-ssh-key"
	admin2FAUser    = "admin-2fa-user"
	admin2FAKey     = "admin-2fa-key"
	admin2FASSHKey  = "admin-2fa-ssh-key"
)

var (
	// counter keeps track of the number of OSLogin tests running.
	counter int

	// test2FAUsers is the list of 2FA test users to use for this test.
	twoFATestUsers = []test2FAUser{
		{
			email:  "normal-2fa-user",
			sshKey: "normal-2fa-ssh-key",
			twoFA:  "normal-2fa-key",
		},
		{
			email:  "normal-2fa-user-1",
			sshKey: "normal-2fa-ssh-key-1",
			twoFA:  "normal-2fa-key-1",
		},
		{
			email:  "normal-2fa-user-2",
			sshKey: "normal-2fa-ssh-key-2",
			twoFA:  "normal-2fa-key-2",
		},
		{
			email:  "normal-2fa-user-3",
			sshKey: "normal-2fa-ssh-key-3",
			twoFA:  "normal-2fa-key-3",
		},
		{
			email:  "normal-2fa-user-4",
			sshKey: "normal-2fa-ssh-key-4",
			twoFA:  "normal-2fa-key-4",
		},
	}

	// twoFAAdminTestUsers is the list of 2FA admin test users to use for this test.
	// Ideally there is one admin test user for every normal test user to form "pairs".
	twoFAAdminTestUsers = []test2FAUser{
		{
			email:  "admin-2fa-user",
			sshKey: "admin-2fa-ssh-key",
			twoFA:  "admin-2fa-key",
		},
		{
			email:  "admin-2fa-user-1",
			sshKey: "admin-2fa-ssh-key-1",
			twoFA:  "admin-2fa-key-1",
		},
		{
			email:  "admin-2fa-user-2",
			sshKey: "admin-2fa-ssh-key-2",
			twoFA:  "admin-2fa-key-2",
		},
		{
			email:  "admin-2fa-user-3",
			sshKey: "admin-2fa-ssh-key-3",
			twoFA:  "admin-2fa-key-3",
		},
		{
			email:  "admin-2fa-user-4",
			sshKey: "admin-2fa-ssh-key-4",
			twoFA:  "admin-2fa-key-4",
		},
	}
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

	normalUser := twoFATestUsers[counter % len(twoFATestUsers)]
	adminUser := twoFAAdminTestUsers[counter % len(twoFAAdminTestUsers)]

	ssh, err := t.CreateTestVM("ssh")
	if err != nil {
		return err
	}
	ssh.AddScope(platformScope)
	ssh.AddMetadata("enable-oslogin", "false")
	ssh.AddMetadata(normal2FAUser, normalUser.email)
	ssh.AddMetadata(normal2FAKey, normalUser.twoFA)
	ssh.AddMetadata(normal2FASSHKey, normalUser.sshKey)
	ssh.AddMetadata(admin2FAUser, adminUser.email)
	ssh.AddMetadata(admin2FAKey, adminUser.twoFA)
	ssh.AddMetadata(admin2FASSHKey, adminUser.sshKey)
	ssh.RunTests("TestOsLoginDisabled|TestSSH|TestAdminSSH|Test2FASSH|Test2FAAdminSSH")

	twofa, err := t.CreateTestVM("twofa")
	if err != nil {
		return err
	}
	twofa.AddScope(computeScope)
	twofa.AddMetadata("enable-oslogin", "true")
	twofa.AddMetadata("enable-oslogin-2fa", "true")
	twofa.RunTests("TestAgent")

	// This is used to stagger the admin users to avoid hitting 2FA quotas.
	counter++
	return nil
}
