//go:build cit
// +build cit

package suspendresume

import (
	"net/http"
	"os"
	"strings"
	"testing"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
)

func TestSuspend(t *testing.T) {
	if utils.IsWindows() {
		out, err := utils.RunPowershellCmd(`$TurnOffSettingCount=0;
 $SleepButtonSettingCount=0;
 Get-CimInstance -Namespace root\cimv2\power -ClassName Win32_PowerSettingDataIndex | ForEach-Object {
   $power_setting = $_ | Get-CimAssociatedInstance -ResultClassName Win32_PowerSetting -OperationTimeoutSec 10;
   if ($power_setting -and $power_setting.ElementName -eq "Turn off display after") {
     if ($_.SettingIndexValue -ne 0) {
       $TurnOffSettingCount=$TurnOffSettingCount+1;
     }
   }
   if ($power_setting -and $power_setting.ElementName -eq "Sleep button action") {
     if ($_.SettingIndexValue -ne 1) {
       $SleepButtonSettingCount=$SleepButtonSettingCount+1;
     }
   }
 };
 Return "TurnOffDisplay:"+$TurnOffSettingCount+" SleepButton:"+$SleepButtonSettingCount;+""`)
		if err != nil {
			t.Errorf("could not check power settings: %s %s %v", out.Stdout, out.Stderr, err)
		}
		if !strings.Contains(out.Stdout, "TurnOffDisplay:0") || !strings.Contains(out.Stdout, "SleepButton:0") {
			t.Errorf("found misconfigured power settings, want 0 each but got %s", out.Stdout)
		}
	}
	marker := "/var/suspend-test-start"
	if utils.IsWindows() {
		marker = `C:\suspend-test-start`
	}
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
	_, err = http.Get("https://cloud.google.com")
	if err != nil {
		t.Errorf("no network connectivity after resume: %v", err)
	}
}
