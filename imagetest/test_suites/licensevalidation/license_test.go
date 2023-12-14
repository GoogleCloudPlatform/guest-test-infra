//go:build cit
// +build cit

package licensevalidation

import (
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

	command := "cscript C:\\Windows\\system32\\slmgr.vbs /dli"
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error getting license status: %v", err)
	}

	if !strings.Contains(output.Stdout, "License Status: Licensed") {
		t.Fatalf("Activation info does not contain 'Licensed': %s", output.Stdout)
	}

}
