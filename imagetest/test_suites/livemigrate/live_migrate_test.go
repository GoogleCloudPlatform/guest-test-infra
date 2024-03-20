//go:build cit
// +build cit

package livemigrate

import (
	"net/http"
	"os"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
)

func TestLiveMigrate(t *testing.T) {
	marker := "/var/lm-test-start"
	if utils.IsWindows() {
		marker = `C:\lm-test-start`
	}
	if _, err := os.Stat(marker); err != nil && !os.IsNotExist(err) {
		t.Fatalf("could not determine if live migrate testing has already started: %v", err)
	} else if err == nil {
		t.Fatal("unexpected reboot during live migrate test")
	}
	err := os.WriteFile(marker, nil, 0777)
	if err != nil {
		t.Fatalf("could not mark beginning of live migrate testing: %v", err)
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
	req := &computepb.SimulateMaintenanceEventInstanceRequest{
		Project:  prj,
		Zone:     zone,
		Instance: inst,
	}
	op, err := client.SimulateMaintenanceEvent(ctx, req)
	if err != nil {
		t.Fatalf("could not migrate self: %v", err)
	}
	op.Wait(ctx) // Errors here come from things completely out of our control, such as the availability of a physical machine to take our VM.
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("could not confirm migrate testing has started ok: %v", err)
	}
	_, err = http.Get("https://cloud.google.com/")
	if err != nil {
		t.Errorf("lost network connection after live migration")
	}
}
