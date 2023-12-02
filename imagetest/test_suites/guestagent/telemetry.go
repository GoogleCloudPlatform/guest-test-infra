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

//go:build cit
// +build cit

package guestagent

import (
	"os/exec"
	"testing"
	"time"
	"path"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func getAgentOutput(t *testing.T) string {
	var cmd *exec.Cmd
	if utils.IsWindows() {
		cmd = exec.Command("powershell.exe", "-NonInteractive", "Get-WinEvent", "-Providername", "GCEGuestAgent")
	} else {
		cmd = exec.Command("journalctl", "-o", "cat", "-eu", "google-guest-agent")
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("could not get agent output: %v", err)
	}
	return string(out)
}

func TestTelemtryEnabled(t *testing.T) {
	if !strings.Contains(getAgentOutput(t), "Successfully scheduled job telemetryJobID") {
		t.Errorf("Telemetry jobs are not scheduled by default")
	}
	utils.PutMetadata(utils.Context(t), path.Join("instance", "attributes", "disable-guest-telemetry"), "true")
	time.Sleep(time.Second)
	if strings.Contains(getAgentOutput(t), "Successfully scheduled job telemetryJobID") {
		t.Errorf("Telemetry jobs are scheduled after setting disable-guest-telemetry=true")
	}
}

func TestTelemtryDisabled(t *testing.T) {
	if strings.Contains(getAgentOutput(t), "Successfully scheduled job telemetryJobID") {
		t.Errorf("Telemetry jobs are scheduled after setting disable-guest-telemetry=true")
	}
	utils.PutMetadata(utils.Context(t), path.Join("instance", "attributes", "disable-guest-telemetry"), "false")
	time.Sleep(time.Second)
	if !strings.Contains(getAgentOutput(t), "Successfully scheduled job telemetryJobID") {
		t.Errorf("Telemetry jobs are not scheduled after setting disable-guest-telemetry=false")
	}
}
