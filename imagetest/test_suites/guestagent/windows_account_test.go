//go:build cit
// +build cit

package packagevalidation

import (
	"fmt"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"strings"
	"testing"
)

// Verifies that a powershell command ran with no errors and exited with an exit code of 0.
// If a username or password was invalid, this should result in a testing error.
// Returns the standard output in case it needs to be used later.
func verifyPowershellCmd(t *testing.T, cmd string) string {
	procStatus, err := utils.RunPowershellCmd(cmd)
	if err != nil {
		t.Fatalf("cmd %s failed: stdout %s, stderr %v, error %v", cmd, procStatus.Stdout, procStatus.Stderr, err)
	}

	stdout := procStatus.Stdout
	if procStatus.Exitcode != 0 {
		t.Fatalf("cmd %s failed with exitcode %d, stdout %s and stderr %s", cmd, procStatus.Exitcode, stdout, procStatus.Stderr)
	}
	return stdout
}

func TestCreateNewWindowsUser(t *testing.T) {
	utils.WindowsOnly(t)
	username := "windowsuser5"
	password := "gyug3q445m0!"

	createUserCmd := fmt.Sprintf("net user %s %s /add", username, password)
	verificationCmd := fmt.Sprintf("Start-Process -Credential (New-Object System.Management.Automation.PSCredential(\"%s\", (\"%s\" | ConvertTo-SecureString -AsPlainText -Force))) -WorkingDirectory C:\\Windows\\System32 -FilePath cmd.exe", username, password)
	listUsersCmd := "Get-CIMInstance Win32_UserAccount | ForEach-Object { Write-Output $_.Name}"

	verifyPowershellCmd(t, createUserCmd)
	verifyPowershellCmd(t, verificationCmd)
	usersList := verifyPowershellCmd(t, listUsersCmd)
	userFound := false
	for _, line := range strings.Split(strings.TrimSpace(usersList), "\n") {
		if username == strings.TrimSpace(line) {
			userFound = true
			break
		}
	}
	if !userFound {
		t.Fatalf("user %s not found after username creation", username)
	}
}
