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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/protobuf/proto"
)

func snapshotTestPrep(t *testing.T) {
	t.Helper()
	if !utils.IsWindows() {
		// Make snapshots directory and write pre and post snapshot scripts
		err := os.MkdirAll("/etc/google/snapshots", 0770)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile("/etc/google/snapshots/pre.sh", []byte("#!/bin/bash\ndate>>/etc/google/snapshots/pre-snapshot-write\n"), 0770)
		if err != nil {
			t.Fatal(err)
		}
		err = os.WriteFile("/etc/google/snapshots/post.sh", []byte("#!/bin/bash\ndate>>/etc/google/snapshots/post-snapshot-write\n"), 0770)
		if err != nil {
			t.Fatal(err)
		}

		// Enable snapshot scripts in the agent and restart it.
		// Wait 5 seconds for connection to snapshot service
		agentcfg, err := os.ReadFile("/etc/default/instance_configs.cfg")
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		agentcfg = append(agentcfg, []byte("[Snapshots]\nenabled = true\ntimeout_in_seconds = 300\n")...)
		err = os.WriteFile("/etc/default/instance_configs.cfg", agentcfg, 0640)
		if err != nil {
			t.Fatal(err)
		}
		err = exec.Command("systemctl", "restart", "google-guest-agent").Run()
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(5) * time.Second)
	}
}

func verifySnapshotSuccess(t *testing.T) {
	t.Helper()
	if utils.IsWindows() {
		// We have no way to communicate with VSSAgent, and polling for the existence
		// of shadows is flakier than just checking its logs.
		//
		res, err := utils.RunPowershellCmd("Get-WinEvent -LogName GCE-VSS-Agent/Operational | Format-List -Property Message")
		if err != nil {
			t.Fatal(err)
		}
		for _, msg := range []string{
			"DoSnapshotSet aysnc operation completed.",
			"Creating the shadow in DoSnapshotSet.",
			"PrepareVolumes return status 0",
			"CheckSelectedWriterStatus returned with 0",
		} {
			if !strings.Contains(res.Stdout, msg) {
				t.Errorf("Could not find message %s in GCE-VSS-Agent logs", msg)
			}
		}
		t.Logf("GCE-VSS-Agent Output: %s", res.Stdout)
	} else {
		// Read pre and post snapshot files, if they don't exist or weren't written to once then the script execution did something we don't expect.
		pre, err := os.ReadFile("/etc/google/snapshots/pre-snapshot-write")
		if err != nil {
			t.Fatal(err)
		}
		post, err := os.ReadFile("/etc/google/snapshots/post-snapshot-write")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(pre), "\n") != 1 {
			t.Errorf("Unexpected number of exections of /etc/google/snapshots/pre.sh, want 1 got %d", strings.Count(string(pre), "\n"))
		}
		if strings.Count(string(post), "\n") != 1 {
			t.Errorf("Unexpected number of exections of /etc/google/snapshots/post.sh, want 1 got %d", strings.Count(string(post), "\n"))
		}
	}
}

func TestSnapshotScripts(t *testing.T) {
	ctx := utils.Context(t)
	snapshotTestPrep(t)

	prj, zone, err := utils.GetProjectZone(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := utils.GetMetadata(ctx, "instance", "name")
	if err != nil {
		t.Fatal(err)
	}

	// Make a snapshot request for the boot disk of this instance
	disksClient, err := compute.NewDisksRESTClient(ctx)
	snapshotsClient, err := compute.NewSnapshotsRESTClient(ctx)
	createReq := &computepb.CreateSnapshotDiskRequest{
		Project:    prj,
		GuestFlush: proto.Bool(true),
		Disk:       inst,
		Zone:       zone,
		SnapshotResource: &computepb.Snapshot{
			Name:       proto.String("snapshot-" + inst),
			SourceDisk: proto.String(fmt.Sprintf("projects/%s/zones/%s/disks/%s", prj, zone, inst)),
		},
	}
	op, err := disksClient.CreateSnapshot(ctx, createReq)
	if err != nil {
		t.Fatalf("unable to create snapshot: %v", err)
	}
	err = op.Wait(ctx)
	if err != nil {
		t.Errorf("failed to wait for snapshot creation: %v", err)
	}

	// Delete it after. No need to wait for deletion
	deleteReq := &computepb.DeleteSnapshotRequest{
		Project:  prj,
		Snapshot: "snapshot-" + inst,
	}
	op, err = snapshotsClient.Delete(ctx, deleteReq)
	if err != nil {
		t.Errorf("unable to delete snapshot: %v", err)
	}

	verifySnapshotSuccess(t)
}
