//go:build cit
// +build cit

package network

import (
	"context"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"github.com/go-ping/ping"
)

func TestPingVMToVM(t *testing.T) {
	ctx := utils.Context(t)
	primaryIP, err := utils.GetMetadata(ctx, "instance", "network-interfaces", "0", "ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, %v", err)
	}
	secondaryIP, err := utils.GetMetadata(ctx, "instance", "network-interfaces", "1", "ip")
	if err != nil {
		t.Fatalf("couldn't get internal network IP from metadata, %v", err)
	}

	name, err := utils.GetRealVMName(vm2Config.name)
	if err != nil {
		t.Fatalf("failed to determine target vm name: %v", err)
	}
	if err := pingTargetRetries(ctx, primaryIP, name); err != nil {
		t.Fatalf("failed to ping remote %s via %s (primary network): %v", name, primaryIP, err)
	}
	if err := pingTargetRetries(ctx, secondaryIP, vm2Config.ip); err != nil {
		t.Fatalf("failed to ping remote %s via %s (secondary network): %v", vm2Config.ip, secondaryIP, err)
	}
}

func pingTargetRetries(ctx context.Context, source, target string) error {
	// Attempt to ping target until context is expired
	for ctx.Err() == nil {
		if pingTarget(source, target) == nil {
			return nil
		}
	}
	return ctx.Err()
}

// send 5 ICMP echo packets and wait a maximum of one second to receieve 5 responses
func pingTarget(source, target string) error {
	pinger, err := ping.NewPinger(target)
	if err != nil {
		return err
	}
	pinger.SetPrivileged(true)
	pinger.Source = source
	pinger.Count = 5
	pinger.Timeout = time.Second
	return pinger.Run()
}
