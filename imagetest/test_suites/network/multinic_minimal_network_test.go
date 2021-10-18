// +build cit

package network

import (
	"net"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestPingVMToVM(t *testing.T) {
	vm3nic0IP, err := utils.GetMetadata("network-interfaces/0/ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, %v", err)
	}

	vmname, err := utils.GetRealVMName(vm4)
	if err != nil {
		t.Fatalf("failed to get vm name: %v", err)
	}
	addr, err := net.LookupIP(vmname)
	if err != nil {
		t.Fatalf("failed to get address for primary interface: %v", err)
	}
	vm4nic0IP := addr[0].String()
	if err := pingTargetRetries(vm3nic0IP, vm4nic0IP); err != nil {
		t.Fatalf("failed to ping remote %s via %s (default network): %v", vm4nic0IP, vm3nic0IP, err)
	}
	if err := pingTargetRetries(vm3IP, vm4IP); err != nil {
		t.Fatalf("failed to ping remote %s via %s (custom network): %v", vm4IP, vm3IP, err)
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
