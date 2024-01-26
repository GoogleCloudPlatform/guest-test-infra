//go:build cit
// +build cit

package packagevalidation

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

type version struct {
	major int
	minor int
}

func (v version) lessThan(check version) bool {
	if v.major < check.major {
		return true
	}
	if v.major == check.major && v.minor < check.minor {
		return true
	}
	return false

}

func TestAutoUpdateEnabled(t *testing.T) {
	utils.WindowsOnly(t)
	command := `$au_path = 'HKLM:\SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU'
    $au = Get-Itemproperty -Path $au_path
    if ($au.NoAutoUpdate -eq 1) {exit 1}
    $au.AUOptions`
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting auto update status: %v", err)
	}
	// AUOptions status values are documented here:
	// https://learn.microsoft.com/de-de/security-updates/windowsupdateservices/18127499
	statusStr := strings.TrimSpace(output.Stdout)
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		t.Fatalf("Status code '%s' is not an integer: %v", statusStr, err)
	}

	if status == 0 {
		t.Fatalf("Windows auto update is not configured. Status code: %d", status)
	}

	if status != 4 {
		t.Fatalf("Windows auto update is not enabled. Status code: %d", status)
	}
}

func TestNetworkConnecton(t *testing.T) {
	utils.WindowsOnly(t)
	command := "Test-Connection www.google.com -Count 1 -ErrorAction stop -quiet"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error testing connection: %v", err)
	}

	conn := strings.TrimSpace(output.Stdout)

	if conn != "True" {
		t.Fatalf("Connection test did not return True: %s", output.Stdout)
	}
}

func TestEmsEnabled(t *testing.T) {
	utils.WindowsOnly(t)
	command := "& bcdedit | Select-String -Quiet -Pattern \"ems * Yes\""
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting ems status: %v", err)
	}

	ems := strings.TrimSpace(output.Stdout)

	if ems != "True" {
		t.Fatalf("Ems value not True: %s", output.Stdout)
	}
}

func TestTimeZoneUTC(t *testing.T) {
	utils.WindowsOnly(t)
	command := "(Get-CimInstance Win32_OperatingSystem).CurrentTimeZone"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting timezone: %v", err)
	}

	if strings.TrimSpace(output.Stdout) != "0" {
		t.Fatalf("Timezone not set to 0. Output: %s", output.Stdout)
	}
}

func TestPowershellVersion(t *testing.T) {
	utils.WindowsOnly(t)
	expectedVersion := version{major: 5, minor: 1}
	var actualVersion version
	command := "$PSVersionTable.PSVersion.Major"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting Powershell version: %v", err)
	}

	actualVersion.major, err = strconv.Atoi(strings.TrimSpace(output.Stdout))
	if err != nil {
		t.Fatalf("Unexpected major version: %s", output.Stdout)
	}

	command = "$PSVersionTable.PSVersion.Minor"
	output, err = utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting Powershell version: %v", err)
	}

	actualVersion.minor, err = strconv.Atoi(strings.TrimSpace(output.Stdout))
	if err != nil {
		t.Fatalf("Unexpected minor version: %s", output.Stdout)
	}

	if actualVersion.lessThan(expectedVersion) {
		t.Fatalf("Powershell version less than %d.%d: %s", expectedVersion.major, expectedVersion.minor, output.Stdout)
	}

}

func TestServerGuiShell(t *testing.T) {
	utils.WindowsOnly(t)
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("could not get image name: %v", err)
	}
	expect := "True"
	if strings.Contains(image, "-core") {
		expect = "False"
	}
	o, err := utils.RunPowershellCmd(`(Get-ItemProperty "HKLM:\\Software\\Microsoft\\Windows NT\\CurrentVersion\\Server\\ServerLevels" -Name Server-Gui-Shell -ErrorAction SilentlyContinue) -ne $null`)
	if err != nil {
		t.Fatalf("could not get Server-Gui-Shell installation state: %v %v", o.Stdout, err)
	}
	installState := strings.TrimSuffix(strings.TrimSuffix(o.Stdout, "\n"), "\r")
	if installState != expect {
		t.Errorf("unexpected Server-Gui-Shell installation state, got %s want %s", installState, expect)
	}
}

func TestStartExe(t *testing.T) {
	utils.WindowsOnly(t)
	command := "Start-Process cmd -Args '/c typeperf \"\\Memory\\Available bytes\"'"
	err := utils.CheckPowershellSuccess(command)
	if err != nil {
		t.Fatalf("Unable to start process: %v", err)
	}

	// Needs a few seconds show up in the process list.
	time.Sleep(5 * time.Second)
	command = "Get-Process"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting processes: %v", err)
	}

	if !strings.Contains(output.Stdout, "typeperf") {
		t.Fatalf("typeperf process does not exist: %v", output.Stdout)
	}

	command = "Stop-Process -Name typeperf"
	err = utils.CheckPowershellSuccess(command)
	if err != nil {
		t.Fatalf("Unable to stop process: %v", err)
	}

	command = "Get-Process"
	output, err = utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting processes: %v", err)
	}

	if strings.Contains(output.Stdout, "typeperf") {
		t.Fatal("typeperf process still exists after killing")
	}

}

func TestDotNETVersion(t *testing.T) {
	utils.WindowsOnly(t)
	expectedVersion := version{major: 4, minor: 7}
	command := "Get-ItemProperty \"HKLM:\\SOFTWARE\\Microsoft\\NET Framework Setup\\NDP\\v4\\Full\" -Name Version | Select-Object -ExpandProperty Version"

	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting .NET version: %v", err)
	}

	verInfo := strings.Split(output.Stdout, ".")
	var actualVersion version
	if len(verInfo) < 2 {
		t.Fatalf("Unexpected version info: %s", output.Stdout)
	}
	actualVersion.major, err = strconv.Atoi(strings.TrimSpace(verInfo[0]))
	if err != nil {
		t.Fatalf("Unexpected major version: %s", verInfo[0])
	}
	actualVersion.minor, err = strconv.Atoi(strings.TrimSpace(verInfo[1]))
	if err != nil {
		t.Fatalf("Unexpected minor version: %s", verInfo[1])
	}

	if actualVersion.lessThan(expectedVersion) {
		t.Fatalf(".NET version less than %d.%d: %s", expectedVersion.major, expectedVersion.minor, output.Stdout)
	}
}

func TestServicesState(t *testing.T) {
	utils.WindowsOnly(t)
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata %v", err)
	}
	services := []string{
		"GCEAgent",
	}
	if !utils.IsWindowsClient(image) {
		services = append(services, "GoogleVssAgent")
		services = append(services, "GoogleVssProvider")
	}
	if !utils.Is32BitWindows(image) {
		services = append(services, "google_osconfig_agent")

	}

	for _, service := range services {
		command := fmt.Sprintf("(Get-Service -Name %s -ErrorAction Stop) | Select-Object Name, Status, StartType", service)
		output, err := utils.RunPowershellCmd(command)
		if err != nil {
			t.Fatalf("Error getting service state: %v", err)
		}

		if !strings.Contains(output.Stdout, "Running") || !strings.Contains(output.Stdout, "Automatic") {
			t.Fatalf("'Running' or 'Automatic not found in service state for %s: %s", service, output.Stdout)
		}
	}
}

func TestWindowsEdition(t *testing.T) {
	utils.WindowsOnly(t)
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata %v", err)
	}
	expectedDatacenter := strings.Contains(image, "-dc-")
	command := "(Get-ComputerInfo).WindowsEditionId"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting ComputerInfo: %v", err)
	}
	actualDatacenter := strings.Contains(output.Stdout, "Datacenter")

	if expectedDatacenter != actualDatacenter {
		t.Fatalf("Image name and image do not have matching edition. Image Name: %s, WindowsEditionId: %s", image, output.Stdout)
	}
}

func TestWindowsCore(t *testing.T) {
	utils.WindowsOnly(t)
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata %v", err)
	}
	expectedCore := strings.Contains(image, "-core-")
	command := "(Get-ComputerInfo).WindowsInstallationType"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting ComputerInfo: %v", err)
	}
	actualCore := strings.Contains(output.Stdout, "Core")

	if expectedCore != actualCore {
		t.Fatalf("Image name and image do not have matching core values. Image Name: %s, WindowsInstallationType: %s", image, output.Stdout)
	}
}
