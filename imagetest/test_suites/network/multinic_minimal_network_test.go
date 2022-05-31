// +build cit
// +build linux_test

package network

import (
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestPingVMToVM(t *testing.T) {
	primaryIP, err := utils.GetMetadata("network-interfaces/0/ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, %v", err)
	}
	secondaryIP, err := utils.GetMetadata("network-interfaces/1/ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, %v", err)
	}

	name, err := utils.GetRealVMName(vm2Name)
	if err != nil {
		t.Fatalf("failed to determine target vm name: %v", err)
	}
	if err := pingTargetRetries(primaryIP, name); err != nil {
		t.Fatalf("failed to ping remote %s via %s (primary network): %v", name, primaryIP, err)
	}
	if err := pingTargetRetries(secondaryIP, vm2IP); err != nil {
		t.Fatalf("failed to ping remote %s via %s (secondary network): %v", vm2IP, secondaryIP, err)
	}
}

// pingTargetRetries pings the target up to retry limit, because remote VM is
// not guaranteed to be up at start of test.
func pingTargetRetries(source, target string) error {
	// 24 (23 + final) retries of 5-second maximum connection attempts for
	// total of 120 seconds.
	for i := 0; i < 23; i++ {
		if err := pingTarget(source, target); err == nil {
			return nil
		}
	}
	return pingTarget(source, target)
}

// pingTarget sends ICMP ping to target from source once a second for 5
// seconds, expecting 5 responses.
func pingTarget(source, target string) error {
	cmd := exec.Command("ping", "-q", "-c", "5", "-I", source, "-w", "5", target)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// dummy test for target VM.
func TestEmptyTest(t *testing.T) {
	t.Log("vm boot successfully")
}
