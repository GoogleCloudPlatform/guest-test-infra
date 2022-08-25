//go:build cit
// +build cit

package windows

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const GOOGET = "C:\\ProgramData\\GooGet\\googet.exe"
const STABLE_REPO = "https://packages.cloud.google.com/yuck/repos/google-compute-engine-stable"

func TestGooGetInstalled(t *testing.T) {
	utils.WindowsOnly(t)
	command := fmt.Sprintf("%s installed googet", GOOGET)
	utils.FailOnPowershellFail(command, "GooGet not installed", t)
}

func TestGooGetAvailable(t *testing.T) {
	utils.WindowsOnly(t)
	command := fmt.Sprintf("%s available googet", GOOGET)
	utils.FailOnPowershellFail(command, "GooGet not available", t)
}

func TestSigned(t *testing.T) {
	utils.WindowsOnly(t)
	utils.Skip32BitWindows(t, "Packages not signed on 32-bit client images.")
	command := fmt.Sprintf("(Get-AuthenticodeSignature %s).Status", GOOGET)
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Get-AuthenticodeSignature returned an error: %v", err)
	}

	if strings.TrimSpace(output.Stdout) != "Valid" {
		t.Fatalf("GooGet package signature is not valid.")
	}
}

func TestRemoveInstall(t *testing.T) {
	utils.WindowsOnly(t)
	pkg := "google-compute-engine-auto-updater"
	command := fmt.Sprintf("%s -noconfirm remove %s", GOOGET, pkg)
	utils.FailOnPowershellFail(command, "Error removing package", t)

	command = fmt.Sprintf("%s installed %s", GOOGET, pkg)
	err := utils.CheckPowershellReturnCode(command, 1)
	if err != nil {
		t.Fatal(err)
	}

	command = fmt.Sprintf("%s -noconfirm install %s", GOOGET, pkg)
	utils.FailOnPowershellFail(command, "Error installing package", t)

	command = fmt.Sprintf("%s installed %s", GOOGET, pkg)
	utils.FailOnPowershellFail(command, "Package not installed", t)
}

func TestRepoManagement(t *testing.T) {
	utils.WindowsOnly(t)
	command := fmt.Sprintf("cmd.exe /c del /Q C:\\ProgramData\\GooGet\\repos\\*")
	utils.FailOnPowershellFail(command, "Error deleting repos", t)

	command = fmt.Sprintf("%s available googet", GOOGET)
	err := utils.CheckPowershellReturnCode(command, 1)
	if err != nil {
		t.Fatal(err)
	}

	command = fmt.Sprintf("%s addrepo gce-stable %s", GOOGET, STABLE_REPO)
	utils.FailOnPowershellFail(command, "Error adding repo", t)

	command = fmt.Sprintf("%s available googet", GOOGET)
	utils.FailOnPowershellFail(command, "GooGet not available", t)

	command = fmt.Sprintf("%s rmrepo gce-stable", GOOGET)
	utils.FailOnPowershellFail(command, "Error removing repo", t)

	command = fmt.Sprintf("%s available googet", GOOGET)
	err = utils.CheckPowershellReturnCode(command, 1)
	if err != nil {
		t.Fatal(err)
	}
}
