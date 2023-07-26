package network

import (
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
)

// Name is the name of the test package. It must match the directory name.
var Name = "network"

const (
	vm1Name    = "vm1"
	vm2Name    = "vm2"
  serverName = "server-vm"
  clientName = "client-vm"
	vm1IP      = "192.168.0.2"
	vm2IP      = "192.168.0.3"
  serverIP   = "192.168.0.4"
  clientIP   = "192.168.0.5"

  serverStartupScript = "gs://machine_family_testing_startup_scripts/netserver_startup.sh"
  clientStartupScript = "gs://machine_family_testing_startup_scripts/netclient_startup.sh"
)

var vm *imagetest.TestVM

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	network1, err := t.CreateNetwork("network-1", false)
	if err != nil {
		return err
	}
	subnetwork1, err := network1.CreateSubnetwork("subnetwork-1", "10.128.0.0/20")
	if err != nil {
		return err
	}
	subnetwork1.AddSecondaryRange("secondary-range", "10.14.0.0/16")
	if err := network1.CreateFirewallRule("allow-icmp-net1", "icmp", nil, []string{"10.128.0.0/20"}); err != nil {
		return err
	}

	network2, err := t.CreateNetwork("network-2", false)
	if err != nil {
		return err
	}
	subnetwork2, err := network2.CreateSubnetwork("subnetwork-2", "192.168.0.0/16")
	if err != nil {
		return err
	}
	if err := network2.CreateFirewallRule("allow-icmp-net2", "icmp", nil, []string{"192.168.0.0/16"}); err != nil {
		return err
	}

	vm1, err := t.CreateTestVM(vm1Name)
	if err != nil {
		return err
	}
	if err := vm1.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := vm1.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := vm1.SetPrivateIP(network2, vm1IP); err != nil {
		return err
	}

  // Create two VMs for GVNIC performance testing.
  serverVm, err := t.CreateTestVM(serverName)
  if err != nil {
          return err
  }
  if err := serverVm.AddCustomNetwork(network1, subnetwork1); err != nil {
		return err
	}
	if err := serverVm.AddCustomNetwork(network2, subnetwork2); err != nil {
		return err
	}
	if err := serverVm.SetPrivateIP(network2, serverIP); err != nil {
		return err
	}
	if err := serverVm.AddAliasIPRanges("10.14.8.0/24", "secondary-range"); err != nil {
		return err
	}
  serverVm.AddMetadata("enable-guest-attributes", "TRUE")
  serverVm.SetStartupScript(serverStartupScript)
	if err := serverVm.Reboot(); err != nil {
		return err
	}

  clientVm, err := t.CreateTestVM(clientName)
  if err != nil {
          return err
  }
  if err := clientVm.AddCustomNetwork(network1, subnetwork1); err != nil {
          return err
  }
  if err := clientVm.AddCustomNetwork(network2, subnetwork2); err != nil {
          return err
  }
  if err := clientVm.SetPrivateIP(network2, clientIP); err != nil {
          return err
  }
  if err := clientVm.AddAliasIPRanges("10.14.8.0/24", "secondary-range"); err != nil {
          return err
  }
  clientVm.AddMetadata("enable-guest-attributes", "TRUE")
  clientVm.AddMetadata("iperftarget", serverIP)
  clientVm.SetStartupScript(clientStartupScript)
  if err := clientVm.Reboot(); err != nil {
          return err
  }

	vm1.RunTests("TestPingVMToVM|TestDHCP|TestDefaultMTU")

	if strings.Contains(t.Image, "debian-10") || strings.Contains(t.Image, "rhel-7-7-sap") || strings.Contains(t.Image, "rhel-8-1-sap") {
		// GVNIC is not supported on some older distros.
		clientVm.RunTests("TestAlias")
	} else {
		clientVm.UseGVNIC()
    serverVm.UseGVNIC()
		clientVm.RunTests("TestAlias|TestGVNIC")
	}
	return nil
}
