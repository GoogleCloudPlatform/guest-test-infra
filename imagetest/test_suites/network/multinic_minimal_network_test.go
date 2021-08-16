// +build cit

package network

import (
	"net"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestVMToVM(t *testing.T) {
	vm3nic0IP, err := utils.GetMetadata("network-interfaces/0/ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, %v", err)
	}

	vmname, err := utils.GetRealVMName(vm4)
	if err != nil {
		t.Fatalf("failed to get real vm name: %v", err)
	}
	addr, err := net.LookupIP(vmname)
	if err != nil {
		t.Fatalf("failed to get target ip for interface 0, %v", err)
	}
	vm4nic0IP := addr[0].String()
	if err := pingTarget(vm3nic0IP, vm4nic0IP); err != nil {
		t.Fatalf("failed to ping remote host %s from source %s, %v", vm4nic0IP, vm3nic0IP, err)
	}
	if err := pingTarget(vm3IP, vm4IP); err != nil {
		t.Fatalf("failed to ping remot host %s from source %s, %v", vm4IP, vm3IP, err)
	}
}

func pingTarget(source, target string) error {
	cmd := exec.Command("ping", "-q", "-c", "5", "-I", source, "-w", "5", target)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func TestEmptyTest(t *testing.T) {
	t.Log("vm boot successfully")
}
