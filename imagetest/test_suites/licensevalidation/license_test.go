//go:build cit
// +build cit

package licensevalidation

import (
	"sort"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestWindowsActivationStatus(t *testing.T) {
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata %v", err)
	}
	if utils.IsWindowsClient(image) {
		t.Skip("Activation status only checked on server images.")
	}

	activationStatus, err := utils.RunPowershellCmd(`(Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\SoftwareProtectionPlatform\Activation').ProductActivationResult`)
	if err != nil {
		t.Fatalf("could not check activation status: %v", err)
	}
	if strings.TrimSpace(activationStatus.Stdout) != "0" {
		t.Errorf("unexpected activation status, got %s want 0", strings.TrimSpace(activationStatus.Stdout))
	}
}

func TestLicenses(t *testing.T) {
	ctx := utils.Context(t)
	elicensecodes, err := utils.GetMetadata(ctx, "instance", "attributes", "expected-license-codes")
	if err != nil {
		t.Fatalf("Failed to get expected licenses: %v", err)
	}
	expectedLicenseCodes := strings.Split(elicensecodes, ",")
	var actualLicenseCodes []string
	licenseNums, err := utils.GetMetadata(ctx, "instance", "licenses")
	if err != nil {
		t.Fatalf("could not get instance licenses: %v", err)
	}
	for _, lnum := range strings.Split(licenseNums, "\n") {
		lnum = strings.TrimSpace(lnum)
		if lnum == "" {
			continue
		}
		id, err := utils.GetMetadata(ctx, "instance", "licenses", lnum, "id")
		if err != nil {
			t.Fatalf("could not get license %s id: %v", lnum, err)
		}
		actualLicenseCodes = append(actualLicenseCodes, id)
	}
	elicenses, err := utils.GetMetadata(ctx, "instance", "attributes", "expected-licenses")
	if err != nil {
		t.Fatalf("Failed to get expected licenses: %v", err)
	}
	expectedLicenses := strings.Split(elicenses, ",")
	alicenses, err := utils.GetMetadata(ctx, "instance", "attributes", "actual-licenses")
	if err != nil {
		t.Fatalf("Failed to get actual licenses: %v", err)
	}
	actualLicenses := strings.Split(alicenses, ",")

	sort.Strings(expectedLicenseCodes)
	sort.Strings(actualLicenseCodes)
	if len(expectedLicenseCodes) != len(actualLicenseCodes) {
		t.Errorf("wrong number of license codes, got %d want %d", len(actualLicenseCodes), len(expectedLicenseCodes))
	}
	for i := range expectedLicenseCodes {
		if expectedLicenseCodes[i] != actualLicenseCodes[i] {
			t.Errorf("unexpected license code at pos %d, got %s want %s", i, expectedLicenseCodes[i], actualLicenseCodes[i])
		}
	}

	sort.Strings(expectedLicenses)
	sort.Strings(actualLicenses)
	if len(expectedLicenses) != len(actualLicenses) {
		t.Errorf("wrong number of licenses, got %d want %d", len(actualLicenses), len(expectedLicenses))
	}
	for i := range expectedLicenses {
		if expectedLicenses[i] != actualLicenses[i] {
			t.Errorf("unexpected license at pos %d, got %s want %s", i, expectedLicenses[i], actualLicenses[i])
		}
	}
}
