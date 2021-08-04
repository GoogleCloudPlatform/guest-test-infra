// +build cit

package network

import (
	"net"
	"os/exec"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestVMToVM(t *testing.T) {
	sourceIP0, err := utils.GetMetadata("network-interfaces/0/ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, err %v", err)
	}

	vmname, err := utils.GetRealVMName(vm4)
	if err != nil {
		t.Fatalf("failed to get real vm name: %v", err)
	}
	targetIP0, err := getTargetIPbyHostname(vmname)
	if err != nil {
		t.Fatalf("failed to get target ip for interface 0 err %v", err)
	}

	if err := pingTarget(sourceIP0, targetIP0); err != nil {
		t.Fatalf("failed to ping remote host %s from source %s, err %v", targetIP0, sourceIP0, err)
	}
	if err := pingTarget(sourceIP, targetIP); err != nil {
		t.Fatalf("failed to ping remot host %s from source %s, err %v", targetIP, sourceIP, err)
	}
}

func getTargetIPbyHostname(hostname string) (string, error) {
	addr, err := net.LookupIP(hostname)
	if err != nil {
		return "", err
	}
	return addr[0].String(), nil
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
