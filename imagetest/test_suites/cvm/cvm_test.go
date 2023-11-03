package cvm

import (
	"os/exec"
	"strings"
	"testing"
)

var sevMsgList = []string{"AMD Secure Encrypted Virtualization (SEV) active", "AMD Memory Encryption Features active: SEV", "Memory Encryption Features active: AMD SEV"}
var sevSnpMsgList = []string{"AMD Secure Encrypted Virtualization (SEV) active", "SEV: SNP guest platform device intitialized", "Memory Encryption Features active: AMD SEV SEV-ES SEV-SNP"}
var tdxMsgList = []string{"Memory Encryption Features active: TDX", "Memory Encryption Features active: Intel TDX"}

func TestSEVEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep SEV").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, msg := range sevMsgList {
		if strings.Contains(string(output), msg) {
			return
		}
	}
	t.Fatal("Error: SEV not active or found")
}

func TestSEVSNPEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep SEV").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, msg := range sevSnpMsgList {
		if strings.Contains(string(output), msg) {
			return
		}
	}
	t.Fatal("Error: SEV not active or found")
}

func TestTDXEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep TDX").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	for _, msg := range tdxMsgList {
		if strings.Contains(string(output), msg) {
			return
		}
	}
	t.Fatal("Error: TDX not active or found")
}
