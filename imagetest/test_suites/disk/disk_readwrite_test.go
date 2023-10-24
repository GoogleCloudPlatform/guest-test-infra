//go:build cit
// +build cit

package disk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestDiskReadWrite(t *testing.T) {
	utils.WindowsOnly(t)
	testFile := "C:\\test.txt"
	newTestFile := "C:\\testnew.txt"
	content := "Test File Content"
	command := fmt.Sprintf("Set-Content %s \"%s\"", testFile, content)
	utils.FailOnPowershellFail(command, "Error writing file", t)

	command = fmt.Sprintf("Move-Item -Force %s %s", testFile, newTestFile)
	utils.FailOnPowershellFail(command, "Error moving file", t)

	command = fmt.Sprintf("Get-Content %s", newTestFile)
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
	if !strings.Contains(output.Stdout, content) {
		t.Fatalf("Moved file does not contain expected content. Expected: '%s', Actual: '%s'", content, output.Stdout)
	}
}
