package cvm

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
)

func TestCVMEnabled(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "sudo dmesg | grep SEV")
	errStream, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	output, err := cmd.Output()
	if err != nil {
		scanner := bufio.NewScanner(errStream)
		errString := ""
		for scanner.Scan() {
			errString += scanner.Text()
		}
		t.Fatalf("Error: %v", errString)
	}
	if !strings.Contains(string(output), "active") {
		t.Fatalf("Error: SEV not active or found")
	}
}
