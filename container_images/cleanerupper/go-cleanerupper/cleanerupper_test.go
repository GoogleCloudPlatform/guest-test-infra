// Copyright 2022 Google Inc. All Rights Reserved.
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

// cleanerupper provides a library of functions to delete gcp resources in a project matching the given deletion policy.
package cleanerupper

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"sort"
	"testing"
	"time"

	osconfigalpha "cloud.google.com/go/osconfig/apiv1alpha"
	osconfig "cloud.google.com/go/osconfig/apiv1beta"
	computeDaisy "github.com/GoogleCloudPlatform/compute-daisy/compute"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/compute/v1"
	osconfigv1alphapb "google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha"
	osconfigpb "google.golang.org/genproto/googleapis/cloud/osconfig/v1beta"
)

func deleteEverything(any) bool { return true }

func deleteNothing(any) bool { return false }

func TestAgePolicy(t *testing.T) {
	testcases := []struct {
		name     string
		time     time.Time
		resource any
		output   bool
	}{
		{
			name:     "Unknown resource",
			time:     time.Now(),
			resource: struct{}{},
			output:   false,
		},
		{
			name:     "Old OSPolicyAssignment",
			time:     time.Now(),
			resource: &osconfigv1alphapb.OSPolicyAssignment{},
			output:   true,
		},
		{
			name:     "Old Guest Policy",
			time:     time.Now(),
			resource: &osconfigpb.GuestPolicy{},
			output:   true,
		},
		{
			name:     "Old Network",
			time:     time.Now(),
			resource: &compute.Network{CreationTimestamp: "1970-01-01T00:00:01+00:00"},
			output:   true,
		},
		{
			name:     "Old Image",
			time:     time.Now(),
			resource: &compute.Image{CreationTimestamp: "1970-01-01T00:00:01+00:00"},
			output:   true,
		},
		{
			name:     "Old Disk",
			time:     time.Now(),
			resource: &compute.Disk{CreationTimestamp: "1970-01-01T00:00:01+00:00"},
			output:   true,
		},
		{
			name:     "Old Machine Image",
			time:     time.Now(),
			resource: &compute.MachineImage{CreationTimestamp: "1970-01-01T00:00:01+00:00"},
			output:   true,
		},
		{
			name:     "Old Snapshot",
			time:     time.Now(),
			resource: &compute.Snapshot{CreationTimestamp: "1970-01-01T00:00:01+00:00"},
			output:   true,
		},
		{
			name:     "Old Instance",
			time:     time.Now(),
			resource: &compute.Instance{CreationTimestamp: "1970-01-01T00:00:01+00:00"},
			output:   true,
		},
		{
			name:     "Keep label in labels",
			time:     time.Now(),
			resource: &compute.Instance{CreationTimestamp: "1970-01-01T00:00:01+00:00", Labels: map[string]string{keepLabel: ""}},
			output:   false,
		},
		{
			name:     "Keep label in name",
			time:     time.Now(),
			resource: &compute.Instance{CreationTimestamp: "1970-01-01T00:00:01+00:00", Name: keepLabel},
			output:   false,
		},
		{
			name:     "Deletion protection enabled",
			time:     time.Now(),
			resource: &compute.Instance{CreationTimestamp: "1970-01-01T00:00:01+00:00", DeletionProtection: true},
			output:   false,
		},
		{
			name:     "Default network",
			time:     time.Now(),
			resource: &compute.Network{CreationTimestamp: "1970-01-01T00:00:01+00:00", Name: "default"},
			output:   false,
		},
		{
			name:     "Keep label in description",
			time:     time.Now(),
			resource: &compute.Instance{CreationTimestamp: "1970-01-01T00:00:01+00:00", Description: keepLabel},
			output:   false,
		},
		{
			name:     "Unexpected timestamp format",
			time:     time.Now(),
			resource: &compute.Instance{CreationTimestamp: "1970-01-01"},
			output:   false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o := AgePolicy(tc.time)(tc.resource)
			if o != tc.output {
				t.Errorf("Unexpected output from AgePolicy(%v)(%v), got %v but want %v", tc.time, tc.resource, o, tc.output)
			}
		})
	}
}

func TestWorkflowPolicy(t *testing.T) {
	testcases := []struct {
		name     string
		wfID     string
		resource any
		output   bool
	}{
		{
			name:     "Unknown resource",
			wfID:     "asdf",
			resource: struct{}{},
			output:   false,
		},
		{
			name:     "Workflow OSPolicyAssignment",
			wfID:     "asdf",
			resource: &osconfigv1alphapb.OSPolicyAssignment{Name: "ospolicy-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Workflow Guest Policy",
			wfID:     "asdf",
			resource: &osconfigpb.GuestPolicy{Name: "guestpolicy-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Workflow Network",
			wfID:     "asdf",
			resource: &compute.Network{Name: "network-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Default Network",
			wfID:     "ault",
			resource: &compute.Network{Name: "default", Description: "created by Daisy in workflow \"ault\" on behalf of root"},
			output:   false,
		},
		{
			name:     "Workflow Image",
			wfID:     "asdf",
			resource: &compute.Image{Name: "image-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Workflow Disk",
			wfID:     "asdf",
			resource: &compute.Disk{Name: "image-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Workflow Machine Image",
			wfID:     "asdf",
			resource: &compute.MachineImage{Name: "machineimage-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Workflow Snapshot",
			wfID:     "asdf",
			resource: &compute.Snapshot{Name: "snapshot-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Workflow Instance",
			wfID:     "asdf",
			resource: &compute.Instance{Name: "instance-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root"},
			output:   true,
		},
		{
			name:     "Keep label in labels",
			wfID:     "asdf",
			resource: &compute.Instance{Name: "instance-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root", Labels: map[string]string{keepLabel: ""}},
			output:   false,
		},
		{
			name:     "Deletion protection enabled",
			wfID:     "asdf",
			resource: &compute.Instance{Name: "instance-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root", DeletionProtection: true},
			output:   false,
		},
		{
			name:     "Keep label in description",
			wfID:     "asdf",
			resource: &compute.Instance{Name: "network-asdf", Description: "created by Daisy in workflow \"asdf\" on behalf of root. do-not-delete"},
			output:   false,
		},
		{
			name:     "Different workflow in description",
			wfID:     "asdf",
			resource: &compute.Instance{Name: "network-asdf", Description: "created by Daisy in workflow \"1234\" on behalf of root."},
			output:   false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o := WorkflowPolicy(tc.wfID)(tc.resource)
			if o != tc.output {
				t.Errorf("Unexpected output from WorkflowPolicy(%v)(%v), got %v but want %v", tc.wfID, tc.resource, o, tc.output)
			}
		})
	}
}

func TestCleanInstances(t *testing.T) {
	_, daisyFake, err := computeDaisy.NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/aggregated/instances?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"Items":{"Instances":{"instances":[{"SelfLink": "projects/test-project/zones/test-zone/instances/test-instance", "Zone":"test-zone"}]}}}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/test-zone/instances/test-instance?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/test-zone/operations//wait?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else {
			w.WriteHeader(555)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		name    string
		clients Clients
		project string
		policy  PolicyFunc
		output  []string
		dryRun  bool
	}{
		{
			name:    "delete everything dry run",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/zones/test-zone/instances/test-instance"},
			dryRun:  true,
		},
		{
			name:    "delete everything",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/zones/test-zone/instances/test-instance"},
			dryRun:  false,
		},
		{
			name:    "delete nothing",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteNothing,
			dryRun:  false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := CleanInstances(tc.clients, tc.project, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanInstances: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanInstances, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanInstances at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

func TestCleanDisks(t *testing.T) {
	_, daisyFake, err := computeDaisy.NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/aggregated/disks?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":{"zones/test-zone":{"disks":[{"SelfLink": "projects/test-project/zones/test-zone/disk/test-disk", "Zone":"test-zone"}]}}}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/test-zone/disks/test-disk?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/zones/test-zone/operations//wait?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else {
			w.WriteHeader(555)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		name    string
		clients Clients
		project string
		policy  PolicyFunc
		output  []string
		dryRun  bool
	}{
		{
			name:    "delete everything dry run",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/zones/test-zone/disks/test-disk"},
			dryRun:  true,
		},
		{
			name:    "delete everything",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/zones/test-zone/disks/test-disk"},
		},
		{
			name:    "delete nothing",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteNothing,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := CleanDisks(tc.clients, tc.project, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanDisks: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanDisks, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanDisks at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

func TestCleanImages(t *testing.T) {
	_, daisyFake, err := computeDaisy.NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/global/images?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":[{"SelfLink": "projects/test-project/global/images/test-image"}]}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/global/images/test-image?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else {
			w.WriteHeader(555)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		name    string
		clients Clients
		project string
		policy  PolicyFunc
		output  []string
		dryRun  bool
	}{
		{
			name:    "delete everything",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/images/test-image"},
		},
		{
			name:    "delete everything dry run",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/images/test-image"},
			dryRun:  true,
		},
		{
			name:    "delete nothing",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteNothing,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := CleanImages(tc.clients, tc.project, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanImages: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanImages, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanImages at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

func TestCleanMachineImages(t *testing.T) {
	_, daisyFake, err := computeDaisy.NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/global/machineImages?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":[{"SelfLink": "projects/test-project/global/machineImages/test-image"}]}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/global/machineImages/test-image?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else {
			w.WriteHeader(555)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		name    string
		clients Clients
		project string
		policy  PolicyFunc
		output  []string
		dryRun  bool
	}{
		{
			name:    "delete everything",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/machineImages/test-image"},
		},
		{
			name:    "delete everything dry drun",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/machineImages/test-image"},
			dryRun:  true,
		},
		{
			name:    "delete nothing",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteNothing,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := CleanMachineImages(tc.clients, tc.project, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanMachineImages: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanMachineImages, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanMachineImages at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

func TestCleanSnapshots(t *testing.T) {
	_, daisyFake, err := computeDaisy.NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/global/snapshots?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":[{"SelfLink": "projects/test-project/global/snapshots/test-snapshot"}]}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/global/snapshots/test-snapshot?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", "test-project") {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"DONE"}`))
		} else {
			w.WriteHeader(555)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		name    string
		clients Clients
		project string
		policy  PolicyFunc
		output  []string
		dryRun  bool
	}{
		{
			name:    "delete everything",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/snapshots/test-snapshot"},
		},
		{
			name:    "delete everything dry run",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/snapshots/test-snapshot"},
			dryRun:  true,
		},
		{
			name:    "delete nothing",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteNothing,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := CleanSnapshots(tc.clients, tc.project, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanShapshots: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanSnapshots, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanSnapshots at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

func TestCleanNetworks(t *testing.T) {
	_, daisyFake, err := computeDaisy.NewTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/global/networks?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":[{"SelfLink": "projects/test-project/global/networks/test-network", "AutoCreateSubnetworks": true}]}`)
		} else if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/global/firewalls?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":[{"Network": "projects/test-project/global/networks/fake-network"}, {"SelfLink": "projects/test-project/global/firewalls/test-firewall", "Network": "projects/test-project/global/networks/test-network"}]}`)
		} else if r.Method == "GET" && r.URL.String() == fmt.Sprintf("/projects/%s/aggregated/subnetworks?alt=json&pageToken=&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"items":{"regions/test-region":{"subnetworks":[{"Network": "projects/test-project/global/networks/fake-network"}, {"Network": "projects/test-project/global/networks/test-network","SelfLink": "projects/test-project/regions/test-region/subnetworks/test-subnetwork", "Name": "test-subnetwork", "Region": "test-region", "IpCidrRange": "10.1.0.0/48"}, {"Network": "projects/test-project/global/networks/test-network","SelfLink": "projects/test-project/regions/test-region/subnetworks/test-subnetwork-2", "Name": "test-subnetwork-2", "Region": "test-region", "IpCidrRange": "10.128.0.0/48"}]}}}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/global/firewalls/test-firewall?alt=json&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/global/networks/test-network?alt=json&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else if r.Method == "DELETE" && r.URL.String() == fmt.Sprintf("/projects/%s/regions/test-region/subnetworks/test-subnetwork?alt=json&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/global/operations//wait?alt=json&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else if r.Method == "POST" && r.URL.String() == fmt.Sprintf("/projects/%s/regions/test-region/operations//wait?alt=json&prettyPrint=false", "test-project") {
			fmt.Fprint(w, `{"Status":"DONE"}`)
		} else {
			w.WriteHeader(555)
			fmt.Fprintln(w, "URL and Method not recognized:", r.Method, r.URL)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	testcases := []struct {
		name    string
		clients Clients
		project string
		policy  PolicyFunc
		output  []string
		dryRun  bool
	}{
		{
			name:    "delete everything dry run",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/firewalls/test-firewall", "projects/test-project/global/networks/test-network", "projects/test-project/regions/test-region/subnetworks/test-subnetwork"},
			dryRun:  true,
		},
		{
			name:    "delete everything",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteEverything,
			output:  []string{"projects/test-project/global/firewalls/test-firewall", "projects/test-project/global/networks/test-network", "projects/test-project/regions/test-region/subnetworks/test-subnetwork"},
		},
		{
			name:    "delete nothing",
			clients: Clients{Daisy: daisyFake},
			project: "test-project",
			policy:  deleteNothing,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := CleanNetworks(tc.clients, tc.project, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanNetworks: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanNetworks, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanNetworks at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

type osconfigFakeClient struct{}

func (osconfigFakeClient) ListGuestPolicies(ctx context.Context, req *osconfigpb.ListGuestPoliciesRequest, opts ...gax.CallOption) *osconfig.GuestPolicyIterator {
	return &osconfig.GuestPolicyIterator{}
}

func (osconfigFakeClient) DeleteGuestPolicy(ctx context.Context, req *osconfigpb.DeleteGuestPolicyRequest, opts ...gax.CallOption) error {
	if path.Base(req.Name) != "test-policy" {
		return fmt.Errorf("unknown policy %s", req.Name)
	}
	return nil
}

func TestDeleteGuestPolicies(t *testing.T) {
	osconfigFake := osconfigFakeClient{}
	testcases := []struct {
		name      string
		clients   Clients
		gpolicies []*osconfigpb.GuestPolicy
		policy    PolicyFunc
		output    []string
		dryRun    bool
	}{
		{
			name:      "delete everything dry run",
			clients:   Clients{OSConfig: osconfigFake},
			gpolicies: []*osconfigpb.GuestPolicy{{Name: "test-policy"}},
			policy:    deleteEverything,
			output:    []string{"test-policy"},
			dryRun:    true,
		},
		{
			name:      "delete everything",
			clients:   Clients{OSConfig: osconfigFake},
			gpolicies: []*osconfigpb.GuestPolicy{{Name: "test-policy"}},
			policy:    deleteEverything,
			output:    []string{"test-policy"},
			dryRun:    false,
		},
		{
			name:      "delete nothing",
			clients:   Clients{OSConfig: osconfigFake},
			gpolicies: []*osconfigpb.GuestPolicy{{Name: "policy1"}},
			policy:    deleteNothing,
			dryRun:    false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := deleteGuestPolicies(context.Background(), tc.clients, tc.gpolicies, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanGuestPolicices: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanGuestPolicies, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanGuestPolicies at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}

type osconfigFakeZonalClient struct{}

func (osconfigFakeZonalClient) ListOSPolicyAssignments(ctx context.Context, req *osconfigv1alphapb.ListOSPolicyAssignmentsRequest, opts ...gax.CallOption) *osconfigalpha.OSPolicyAssignmentIterator {
	return &osconfigalpha.OSPolicyAssignmentIterator{}
}

func (osconfigFakeZonalClient) DeleteOSPolicyAssignment(ctx context.Context, req *osconfigv1alphapb.DeleteOSPolicyAssignmentRequest, opts ...gax.CallOption) (*osconfigalpha.DeleteOSPolicyAssignmentOperation, error) {
	if path.Base(req.Name) != "test-policy" {
		return nil, fmt.Errorf("unknown policy %s", req.Name)
	}
	return &osconfigalpha.DeleteOSPolicyAssignmentOperation{}, nil
}

func TestDeleteOSPolicies(t *testing.T) {
	osconfigFake := osconfigFakeZonalClient{}
	testcases := []struct {
		name       string
		clients    Clients
		ospolicies []*osconfigv1alphapb.OSPolicyAssignment
		policy     PolicyFunc
		output     []string
		dryRun     bool
	}{
		{
			name:       "delete everything dry run",
			clients:    Clients{OSConfigZonal: osconfigFake},
			ospolicies: []*osconfigv1alphapb.OSPolicyAssignment{{Name: "test-policy"}},
			policy:     deleteEverything,
			output:     []string{"test-policy"},
			dryRun:     true,
		},
		{
			name:       "delete nothing",
			clients:    Clients{OSConfigZonal: osconfigFake},
			ospolicies: []*osconfigv1alphapb.OSPolicyAssignment{{Name: "policy1"}},
			policy:     deleteNothing,
			dryRun:     false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o, errs := deleteOSPolicies(context.Background(), tc.clients, tc.ospolicies, tc.policy, tc.dryRun)
			if len(errs) > 0 {
				for _, e := range errs {
					t.Errorf("error from CleanGuestPolicices: %v", e)
				}
			}
			if len(o) != len(tc.output) {
				t.Fatalf("unexpected output length from CleanGuestPolicies, want %d but got %d", len(tc.output), len(o))
			}
			sort.Strings(o)
			for i := range o {
				if o[i] != tc.output[i] {
					t.Errorf("unexpected output from CleanGuestPolicies at position %d, want %s but got %s", i, tc.output[i], o[i])
				}
			}
		})
	}
}
