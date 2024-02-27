//go:build cit
// +build cit

package imageboot

import (
	"os"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
)

func TestSuspend(t *testing.T) {
	marker := "/var/suspend-test-start"
	if _, err := os.Stat(marker); err != nil && !os.IsNotExist(err) {
		t.Fatalf("could not determine if suspend testing has already started: %v", err)
	} else if err == nil {
		t.Fatal("unexpected reboot during suspend test")
	}
	err := os.WriteFile(marker, nil, 0777)
	if err != nil {
		t.Fatalf("could not mark beginning of suspend testing: %v", err)
	}
	ctx := utils.Context(t)
	prj, zone, err := utils.GetProjectZone(ctx)
	if err != nil {
		t.Fatalf("could not find project and zone: %v", err)
	}
	inst, err := utils.GetInstanceName(ctx)
	if err != nil {
		t.Fatalf("could not get instance: %v", err)
	}
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		t.Fatalf("could not make compute api client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	req := &computepb.SuspendInstanceRequest{
		Project:  prj,
		Zone:     zone,
		Instance: inst,
	}
	op, err := client.Suspend(ctx, req)
	if err != nil {
		t.Fatalf("could not suspend self: %v", err)
	}
	op.Wait(ctx) // We can't really check the error here, we want to attempt to wait until its suspended but the wait operation will likely error out due to being interrupted by the suspension
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("could not confirm suspend testing has started ok: %v", err)
	}
}

func TestResume(t *testing.T) {
	target, err := utils.GetRealVMName("suspend")
	if err != nil {
		t.Fatalf("could not get target name: %v", err)
	}
	ctx := utils.Context(t)
	prj, zone, err := utils.GetProjectZone(ctx)
	if err != nil {
		t.Fatalf("could not find project and zone: %v", err)
	}
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		t.Fatalf("could not make compute api client: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	for {
		if ctx.Err() != nil {
			t.Fatalf("test ended before instance %s was suspended: %v", target, err)
		}
		req := &computepb.GetInstanceRequest{
			Project:  prj,
			Zone:     zone,
			Instance: target,
		}
		instance, err := client.Get(ctx, req)
		if err == nil && *instance.Status == "SUSPENDED" {
			break
		}
		time.Sleep(time.Second * 2)
	}
	req := &computepb.ResumeInstanceRequest{
		Project:  prj,
		Zone:     zone,
		Instance: target,
	}
	_, err = client.Resume(ctx, req)
	if err != nil {
		t.Fatalf("could not resume target: %v", err)
	}
}
