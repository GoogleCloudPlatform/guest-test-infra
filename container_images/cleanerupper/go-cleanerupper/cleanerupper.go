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

// Package cleanerupper provides a library of functions to delete gcp resources in a project matching the given deletion policy.
package cleanerupper

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	osconfigV1alpha "cloud.google.com/go/osconfig/apiv1alpha"
	osconfig "cloud.google.com/go/osconfig/apiv1beta"
	daisyCompute "github.com/GoogleCloudPlatform/compute-daisy/compute"
	"github.com/googleapis/gax-go/v2"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	osconfigv1alphapb "google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha"
	osconfigpb "google.golang.org/genproto/googleapis/cloud/osconfig/v1beta"
)

const keepLabel = "do-not-delete"

// Clients contains all of the clients needed by cleanerupper functions.
type Clients struct {
	Daisy         daisyCompute.Client
	OSConfig      osconfigInterface
	OSConfigZonal osconfigZonalInterface
}

type osconfigInterface interface {
	ListGuestPolicies(context.Context, *osconfigpb.ListGuestPoliciesRequest, ...gax.CallOption) *osconfig.GuestPolicyIterator
	DeleteGuestPolicy(context.Context, *osconfigpb.DeleteGuestPolicyRequest, ...gax.CallOption) error
}

type osconfigZonalInterface interface {
	ListOSPolicyAssignments(context.Context, *osconfigv1alphapb.ListOSPolicyAssignmentsRequest, ...gax.CallOption) *osconfigV1alpha.OSPolicyAssignmentIterator
	DeleteOSPolicyAssignment(context.Context, *osconfigv1alphapb.DeleteOSPolicyAssignmentRequest, ...gax.CallOption) (*osconfigV1alpha.DeleteOSPolicyAssignmentOperation, error)
}

// NewClients initializes a struct of Clients for use by CleanX functions
func NewClients(ctx context.Context, opts ...option.ClientOption) (*Clients, error) {
	var c Clients
	var err error
	c.Daisy, err = daisyCompute.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	c.OSConfig, err = osconfig.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	c.OSConfigZonal, err = osconfigV1alpha.NewOsConfigZonalClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// PolicyFunc describes a function which takes a some resource and returns a bool indicating whether it should be deleted.
type PolicyFunc func(any) bool

// AgePolicy takes a time.Time and returns a PolicyFunc which indicates to delete anything older than the given time.
// Also contains safeguards such as refusing to delete default networks or resources with a "do-not-delete" label.
func AgePolicy(t time.Time) PolicyFunc {
	return func(resource any) bool {
		var labels map[string]string
		var desc, name string
		var created time.Time
		var err error
		switch r := resource.(type) {
		case *osconfigv1alphapb.OSPolicyAssignment:
			name = r.Name
			desc = r.Description
			created = time.Unix(r.GetRevisionCreateTime().GetSeconds(), int64(r.GetRevisionCreateTime().GetNanos()))
		case *osconfigpb.GuestPolicy:
			name = r.Name
			desc = r.Description
			created = time.Unix(r.GetCreateTime().GetSeconds(), int64(r.GetCreateTime().GetNanos()))
		case *compute.Network:
			name = r.Name
			desc = r.Description
			if r.Name == "default" || strings.Contains(r.Description, "delete") {
				return false
			}
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.MachineImage:
			name = r.Name
			desc = r.Description
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Disk:
			name = r.Name
			desc = r.Description
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Image:
			name = r.Name
			desc = r.Description
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Snapshot:
			name = r.Name
			desc = r.Description
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		case *compute.Instance:
			name = r.Name
			desc = r.Description
			if r.DeletionProtection {
				return false
			}
			labels = r.Labels
			created, err = time.Parse(time.RFC3339, r.CreationTimestamp)
		default:
			return false
		}
		if err != nil {
			return false
		}
		if _, keep := labels[keepLabel]; keep {
			return false
		}
		return t.After(created) && !strings.Contains(desc, keepLabel) && !strings.Contains(name, keepLabel)
	}
}

// WorkflowPolicy takes a daisy workflow ID and returns a PolicyFunc which indicates to delete anything which appears to have been created by this workflow.
// Note that daisy does have its own resource deletion hooks, this is used in edge cases where workflow deletion hooks are unreliable.
// Also contains safeguards such as refusing to delete default networks or resources with a "do-not-delete" label.
func WorkflowPolicy(id string) PolicyFunc {
	return func(resource any) bool {
		var name, desc string
		var labels map[string]string
		switch r := resource.(type) {
		case *osconfigv1alphapb.OSPolicyAssignment:
			name = r.Name
			desc = r.Description
		case *osconfigpb.GuestPolicy:
			name = r.Name
			desc = r.Description
		case *compute.Network:
			if r.Name == "default" {
				return false
			}
			name = r.Name
			desc = r.Description
		case *compute.MachineImage:
			name = r.Name
			desc = r.Description
		case *compute.Disk:
			labels = r.Labels
			name = r.Name
			desc = r.Description
		case *compute.Image:
			labels = r.Labels
			name = r.Name
			desc = r.Description
		case *compute.Snapshot:
			labels = r.Labels
			name = r.Name
			desc = r.Description
		case *compute.Instance:
			if r.DeletionProtection {
				return false
			}
			labels = r.Labels
			name = r.Name
			desc = r.Description
		default:
			return false
		}
		if _, keep := labels[keepLabel]; keep {
			return false
		}
		return strings.HasSuffix(name, id) && strings.Contains(desc, "created by Daisy in workflow") && !strings.Contains(desc, keepLabel)
	}
}

// CleanInstances deletes all instances indicated, returning a slice of deleted instance partial URLs and a slice of errors encountered. On dry run, returns what would have been deleted.
func CleanInstances(clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	instances, err := clients.Daisy.AggregatedListInstances(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing instance in project %q: %v", project, err)}
	}

	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, i := range instances {
		if !delete(i) {
			continue
		}

		zone := path.Base(i.Zone)
		name := path.Base(i.SelfLink)
		partial := fmt.Sprintf("projects/%s/zones/%s/instances/%s", project, zone, name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !dryRun {
				if err := clients.Daisy.DeleteInstance(project, zone, name); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, partial)
		}()
	}
	wg.Wait()
	return deleted, errs
}

// CleanDisks deletes all disks indicated, returning a slice of deleted partial urls and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanDisks(clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	disks, err := clients.Daisy.AggregatedListDisks(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing disks in project %q: %v", project, err)}
	}

	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, d := range disks {
		if !delete(d) {
			continue
		}

		zone := path.Base(d.Zone)
		name := path.Base(d.SelfLink)
		partial := fmt.Sprintf("projects/%s/zones/%s/disks/%s", project, zone, name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !dryRun {
				if err := clients.Daisy.DeleteDisk(project, zone, name); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, partial)
		}()
	}
	wg.Wait()
	return deleted, errs
}

// CleanImages deletes all images indicated, returning a slice of deleted partial urls and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanImages(clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	images, err := clients.Daisy.ListImages(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing images in project %q: %v", project, err)}
	}

	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, d := range images {
		if !delete(d) {
			continue
		}

		name := path.Base(d.SelfLink)
		partial := fmt.Sprintf("projects/%s/global/images/%s", project, name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !dryRun {
				if err := clients.Daisy.DeleteImage(project, name); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, partial)
		}()
	}
	wg.Wait()
	return deleted, errs
}

// CleanMachineImages deletes all machine images indicated, returning a slice of deleted partial urls and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanMachineImages(clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	images, err := clients.Daisy.ListMachineImages(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing machine images in project %q: %v", project, err)}
	}

	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, d := range images {
		if !delete(d) {
			continue
		}

		name := path.Base(d.SelfLink)
		partial := fmt.Sprintf("projects/%s/global/machineImages/%s", project, name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !dryRun {
				if err := clients.Daisy.DeleteMachineImage(project, name); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, partial)
		}()
	}
	wg.Wait()
	return deleted, errs
}

// CleanSnapshots deletes all snapshots indicated, returning a slice of deleted partial urls and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanSnapshots(clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	images, err := clients.Daisy.ListSnapshots(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing snapshots in project %q: %v", project, err)}
	}

	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, d := range images {
		if !delete(d) {
			continue
		}

		name := path.Base(d.SelfLink)
		partial := fmt.Sprintf("projects/%s/global/snapshots/%s", project, name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !dryRun {
				if err := clients.Daisy.DeleteSnapshot(project, name); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, partial)
		}()
	}
	wg.Wait()
	return deleted, errs
}

// CleanNetworks deletes all networks indicated, as well as all subnetworks and firewall rules that are part of the network indicated for deleted. Returns a slice of deleted partial urls and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanNetworks(clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	networks, err := clients.Daisy.ListNetworks(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing networks in project %q: %v", project, err)}
	}

	firewalls, err := clients.Daisy.ListFirewallRules(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing firewalls in project %q: %v", project, err)}
	}

	subnetworks, err := clients.Daisy.AggregatedListSubnetworks(project)
	if err != nil {
		return nil, []error{fmt.Errorf("error listing subnetworks in project %q: %v", project, err)}
	}

	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	var wg sync.WaitGroup
	for _, n := range networks {
		if !delete(n) {
			continue
		}

		name := path.Base(n.SelfLink)
		netpartial := fmt.Sprintf("projects/%s/global/networks/%s", project, name)
		for _, f := range firewalls {
			if f.Network != n.SelfLink {
				continue
			}
			name := path.Base(f.SelfLink)
			fwallpartial := fmt.Sprintf("projects/%s/global/firewalls/%s", project, name)
			wg.Add(1)
			go func() {
				defer wg.Done()
				if !dryRun {
					if err := clients.Daisy.DeleteFirewallRule(project, name); err != nil {
						errsMu.Lock()
						defer errsMu.Unlock()
						errs = append(errs, err)
						return
					}
				}
				deletedMu.Lock()
				defer deletedMu.Unlock()
				deleted = append(deleted, fwallpartial)
			}()
		}

		for _, sn := range subnetworks {
			if sn.Network != n.SelfLink {
				continue
			}
			// If this network is setup with auto subnetworks we need to ignore any subnetworks that are in 10.128.0.0/9.
			// https://cloud.google.com/vpc/docs/vpc#ip-ranges
			if n.AutoCreateSubnetworks == true {
				i, err := strconv.Atoi(strings.Split(sn.IpCidrRange, ".")[1])
				if err != nil {
					fmt.Printf("Error parsing network range %q: %v\n", sn.IpCidrRange, err)
				}
				if i >= 128 {
					continue
				}
			}

			region := path.Base(sn.Region)
			subnetpartial := fmt.Sprintf("projects/%s/regions/%s/subnetworks/%s", project, region, sn.Name)
			wg.Add(1)
			go func(snName string) {
				defer wg.Done()
				if !dryRun {
					if err := clients.Daisy.DeleteSubnetwork(project, region, snName); err != nil {
						errsMu.Lock()
						defer errsMu.Unlock()
						errs = append(errs, err)
						return
					}
				}
				deletedMu.Lock()
				defer deletedMu.Unlock()
				deleted = append(deleted, subnetpartial)
			}(sn.Name)
		}
		wg.Wait()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !dryRun {
				if err := clients.Daisy.DeleteNetwork(project, name); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, netpartial)
		}()
	}
	wg.Wait()
	return deleted, errs
}

// CleanGuestPolicies deletes all guest policies indicated, returning a slice of deleted policy names and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanGuestPolicies(ctx context.Context, clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	gpolicies, err := getGuestPolicies(ctx, clients, project)
	if err != nil {
		return nil, []error{err}
	}
	return deleteGuestPolicies(ctx, clients, gpolicies, delete, dryRun)
}

func deleteGuestPolicies(ctx context.Context, clients Clients, gpolicies []*osconfigpb.GuestPolicy, delete PolicyFunc, dryRun bool) ([]string, []error) {
	var wg sync.WaitGroup
	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	for _, gp := range gpolicies {
		if !delete(gp) {
			continue
		}
		partial := fmt.Sprintf("%s", gp.GetName())
		wg.Add(1)
		go func(gp *osconfigpb.GuestPolicy) {
			defer wg.Done()
			if !dryRun {
				if err := clients.OSConfig.DeleteGuestPolicy(ctx, &osconfigpb.DeleteGuestPolicyRequest{Name: gp.GetName()}); err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, partial)
		}(gp)
	}
	wg.Wait()
	return deleted, errs
}

func getGuestPolicies(ctx context.Context, clients Clients, project string) (gpolicies []*osconfigpb.GuestPolicy, err error) {
	var gp *osconfigpb.GuestPolicy
	itr := clients.OSConfig.ListGuestPolicies(ctx, &osconfigpb.ListGuestPoliciesRequest{Parent: "projects/" + project})
	for {
		gp, err = itr.Next()
		if err != nil {
			if err == iterator.Done {
				err = nil
			}
			return
		}
		gpolicies = append(gpolicies, gp)
	}
}

func getOSPolicies(ctx context.Context, clients Clients, project string) (ospolicies []*osconfigv1alphapb.OSPolicyAssignment, errs []error) {
	zones, err := clients.Daisy.ListZones(project)
	if err != nil {
		return nil, []error{err}
	}
	var osp *osconfigv1alphapb.OSPolicyAssignment
	for _, zone := range zones {
		itr := clients.OSConfigZonal.ListOSPolicyAssignments(ctx, &osconfigv1alphapb.ListOSPolicyAssignmentsRequest{Parent: fmt.Sprintf("projects/%s/locations/%s", project, zone.Name)})
		for {
			osp, err = itr.Next()
			if err != nil {
				if err != iterator.Done {
					errs = append(errs, err)
				}
				break
			}
			ospolicies = append(ospolicies, osp)
		}
	}
	return
}

func deleteOSPolicies(ctx context.Context, clients Clients, ospolicies []*osconfigv1alphapb.OSPolicyAssignment, delete PolicyFunc, dryRun bool) ([]string, []error) {
	var wg sync.WaitGroup
	var deletedMu sync.Mutex
	var deleted []string
	var errsMu sync.Mutex
	var errs []error
	for _, osp := range ospolicies {
		if !delete(osp) {
			continue
		}
		wg.Add(1)
		go func(osp *osconfigv1alphapb.OSPolicyAssignment) {
			defer wg.Done()
			if !dryRun {
				op, err := clients.OSConfigZonal.DeleteOSPolicyAssignment(ctx, &osconfigv1alphapb.DeleteOSPolicyAssignmentRequest{Name: osp.GetName()})
				if err != nil {
					errsMu.Lock()
					defer errsMu.Unlock()
					errs = append(errs, err)
					return
				}
				op.Wait(ctx)
			}
			deletedMu.Lock()
			defer deletedMu.Unlock()
			deleted = append(deleted, osp.Name)
		}(osp)
	}
	wg.Wait()
	return deleted, errs
}

// CleanOSPolicyAssignments deletes all OS policy assignments indicated, returning a slice of deleted policy assignment names and a slice of encountered errors. On dry run, returns what would have been deleted.
func CleanOSPolicyAssignments(ctx context.Context, clients Clients, project string, delete PolicyFunc, dryRun bool) ([]string, []error) {
	ospolicies, errs := getOSPolicies(ctx, clients, project)
	deleted, deleteerrs := deleteOSPolicies(ctx, clients, ospolicies, delete, dryRun)
	return deleted, append(errs, deleteerrs...)
}
