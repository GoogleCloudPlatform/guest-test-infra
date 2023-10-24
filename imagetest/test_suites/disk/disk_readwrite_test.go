//go:build cit
// +build cit

package disk

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestDiskReadWrite(t *testing.T) {
	if runtime.GOOS == "linux" {
		testDiskReadWriteLinux(t)
	} else {
		testDiskReadWriteWindows(t)
	}
}

func testDiskReadWriteLinux(t *testing.T) {
	testFile := "/test.txt"
	newTestFile := "/testnew.txt"
	content := "Test File Content"
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create file at path %s: error %v", testFile, err)
	}
	_, err = f.WriteString(content)
	if err != nil {
		t.Fatalf("failed to write to file: err %v", err)
	}
	if err = os.Rename(testFile, newTestFile); err != nil {
		t.Fatalf("failed to move file: err %v", err)
	}

	renamedFileBytes, err := os.ReadFile(newTestFile)
	if err != nil {
		t.Fatalf("failed to read contents of new file: error %v", err)
	}

	renamedFileContents := string(renamedFileBytes)
	if renamedFileContents != content {
		t.Fatalf("Moved file does not contain expected content. Expected: '%s', Actual: '%s'", content, renamedFileContents)
	}
}

func testDiskReadWriteWindows(t *testing.T) {
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
