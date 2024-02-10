//go:build cit
// +build cit

package winrm

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func runOrFail(t *testing.T, cmd, msg string) {
	out, err := utils.RunPowershellCmd(cmd)
	if err != nil {
		t.Fatalf("%s: %s %s %v", msg, out.Stdout, out.Stderr, msg)
	}
}

func TestWaitForWinrmConnection(t *testing.T) {
	ctx := utils.Context(t)
	passwd, err := utils.GetMetadata(ctx, "instance", "attributes", "winrm-passwd")
	if err != nil {
		t.Fatalf("could not fetch winrm password: %v", err)
	}
	passwd = strings.TrimSpace(passwd)
	runOrFail(t, fmt.Sprintf(`net user "%s" "%s" /add`, user, passwd), fmt.Sprintf("could not add user %s", user))
	runOrFail(t, fmt.Sprintf(`Add-LocalGroupMember -Group Administrators -Member "%s"`, user), fmt.Sprintf("could not add user %s to administrators", user))
	t.Logf("winrm target boot succesfully at %d", time.Now().UnixNano())
}

func TestWinrmConnection(t *testing.T) {
	ctx := utils.Context(t)
	target, err := utils.GetRealVMName("server")
	if err != nil {
		t.Fatalf("could not get target name: %v", err)
	}
	passwd, err := utils.GetMetadata(ctx, "instance", "attributes", "winrm-passwd")
	if err != nil {
		t.Fatalf("could not fetch winrm password: %v", err)
	}
	passwd = strings.TrimSpace(passwd)
	runOrFail(t, fmt.Sprintf(`winrm set winrm/config/client '@{TrustedHosts="%s"}'`, target), "could not trust target")
	for {
		if ctx.Err() != nil {
			t.Fatalf("test context expired before winrm was available: %v", ctx.Err())
		}
		_, err := utils.RunPowershellCmd(fmt.Sprintf(`Test-WSMan "%s"`, target))
		time.Sleep(time.Minute) // Sleep even on success as there is some delay between target starting winrm and creating the test user
		if err == nil {
			break
		}
	}
	out, err := utils.RunPowershellCmd(fmt.Sprintf(`Invoke-Command -SessionOption(New-PSSessionOption -SkipCACheck -SkipCNCheck -SkipRevocationCheck) -ScriptBlock{ hostname } -ComputerName %s -Credential (New-Object -TypeName System.Management.Automation.PSCredential -ArgumentList "%s\%s", (ConvertTo-SecureString -String '%s' -AsPlainText -Force))`, target, target, user, passwd))
	if err != nil {
		t.Errorf("could not run remote powershell command: %s %s %v", out.Stdout, out.Stderr, err)
	}
	if !strings.Contains(out.Stdout, "server-winrm") {
		t.Errorf("unexpected hostname from remote powershell command, got %s want something that contains server-winrm", out.Stdout)
	}
}
