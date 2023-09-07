package cvm

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestCVMEnabled(t *testing.T) {
	output, err := exec.Command("sudo", "dmesg", "|", "grep", "SEV").Output()
	if !string.Contains(output, "active") {
		t.Fatalf("Error: SEV not active or found")
	}
}
