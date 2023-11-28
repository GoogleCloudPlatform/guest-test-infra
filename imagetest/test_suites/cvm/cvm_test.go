package cvm

import (
	"os/exec"
	"strings"
	"testing"
)

var sevMsgList = []string{"AMD Secure Encrypted Virtualization (SEV) active", "AMD Memory Encryption Features active: SEV", "Memory Encryption Features active: AMD SEV"}
var sevSnpMsgList = []string{"SEV: SNP guest platform device intitialized", "Memory Encryption Features active: AMD SEV SEV-ES SEV-SNP"}
var tdxMsgList = []string{"Memory Encryption Features active: TDX", "Memory Encryption Features active: Intel TDX"}

func searchDmesg(t *testing.T, matches []string) {
	output, err := exec.Command("dmesg").CombinedOutput()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, m := range sevMsgList {
		if strings.Contains(string(output), m) {
			return
		}
	}
	t.Fatal("Module not active or found")
}

func TestSEVEnabled(t *testing.T) {
	searchDmesg(t, sevMsgList)
}

func TestSEVSNPEnabled(t *testing.T) {
	searchDmesg(t, sevSnpMsgList)
}

func TestTDXEnabled(t *testing.T) {
	searchDmesg(t, tdxMsgList)
}
