package cvm

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCVMEnabled(t *testing.T) {
	output, err := exec.Command("/bin/sh", "-c", "sudo dmesg | grep SEV").Output()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if !strings.Contains(string(output), "active") {
		t.Fatalf("Error: SEV not active or found")
	}
}
