package cvm

import (
	"context"
	"os"
	"path"
	"os/exec"
	"strings"
	"testing"

	"google.golang.org/api/compute/v1"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var sevMsgList = []string{"AMD Secure Encrypted Virtualization (SEV) active", "AMD Memory Encryption Features active: SEV", "Memory Encryption Features active: AMD SEV"}
var sevSnpMsgList = []string{"AMD Secure Encrypted Virtualization (SEV) active", "SEV: SNP guest platform device intitialized", "Memory Encryption Features active: AMD SEV SEV-ES SEV-SNP"}
var tdxMsgList = []string{"Memory Encryption Features active: TDX", "Memory Encryption Features active: Intel TDX"}

func TestLiveMigrate(t *testing.T) {
	b, err := os.ReadFile(path.Join(os.TmpDir(), "simulate-maintenance-event-started"))
	if err == nil && string(b) == "1" {
		t.Fatal("Rebooted during live migration")
	} else if !os.IsNotExist(err) {
		t.Fatal("Could not check if maintenance event was already triggered: %v", err)
	}
	ctx := utils.Context(t)
	prj, err := utils.GetMetadata(ctx, "project", "project-id")
	if err != nil {
		t.Fatal(err)
	}
	zone, err := utils.GetMetadata(ctx, "instance", "zone")
	if err != nil {
		t.Fatal(err)
	}
	zone = strings.Split(zone, "/")[len(strings.Split(zone, "/"))-1]
	inst, err := utils.GetMetadata(ctx, "instance", "name")
	if err != nil {
		t.Fatal(err)
	}
	s, err := compute.NewService(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	is := compute.NewInstancesService(s)
	zs := compute.NewZoneOperationsService(s)
	err := os.WriteFile(path.Join(os.TmpDir(), "simulate-maintenance-event-started")), []byte("1"))
	if err != nil {
		t.Fatal(err)
	}
	op, err := is.SimulateMaintenanceEvent(prj, zone, inst).Context(ctx).Do()
	if err != nil {
		t.Fatal(err)
	}
	for time.Tick(time.Duration(10)*time.Second) {
		// https://cloud.google.com/compute/docs/metadata/getting-live-migration-notice#query_the_maintenance_event_metadata_key
		event, err := utils.GetMetadata(ctx, "instance", "maintenance-event")
		if event == "NONE" {
			break
		}
	}
}

func TestSEVEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep SEV").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, msg := range sevMsgList {
		if strings.Contains(string(output), msg) {
			return
		}
	}
	t.Fatal("Error: SEV not active or found")
}

func TestSEVSNPEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep SEV").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, msg := range sevSnpMsgList {
		if strings.Contains(string(output), msg) {
			return
		}
	}
	t.Fatal("Error: SEV not active or found")
}

func TestTDXEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep TDX").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, msg := range tdxMsgList {
		if strings.Contains(string(output), msg) {
			return
		}
	}
	t.Fatal("Error: TDX not active or found")
}
