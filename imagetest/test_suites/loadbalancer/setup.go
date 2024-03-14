package loadbalancer

import (
	"github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"google.golang.org/api/compute/v1"
)

var (
	// Name is the name of the test package. It must match the directory name.
	Name         = "loadbalancer"
	l3IlbIP4Addr = "10.1.2.100"
	l7IlbIP4Addr = "10.1.2.101"

	l3backendVM1IP4addr = "10.1.2.10"
	l3backendVM2IP4addr = "10.1.2.20"
	l7backendVM1IP4addr = "10.1.2.30"
	l7backendVM2IP4addr = "10.1.2.40"

	l3clientVMip4addr = "10.1.2.50"
	l7clientVMip4addr = "10.1.2.60"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	lbnet, err := t.CreateNetwork("loadbalancer", false)
	if err != nil {
		return err
	}
	proxysubnet, err := lbnet.CreateSubnetwork("lb-proxy-subnet", "10.1.2.128/25")
	if err != nil {
		return err
	}
	proxysubnet.SetPurpose("REGIONAL_MANAGED_PROXY")
	proxysubnet.SetRole("ACTIVE")
	lbsubnet, err := lbnet.CreateSubnetwork("lb-backend-subnet", "10.1.2.0/25")
	if err != nil {
		return err
	}
	if err := lbnet.CreateFirewallRule("fw-allow-health-check", "tcp", nil, []string{"130.211.0.0/22", "35.191.0.0/16"}); err != nil {
		return err
	}
	if err := lbnet.CreateFirewallRule("fw-lb-access", "tcp", nil, []string{"10.1.2.0/25"}); err != nil {
		return err
	}
	if err := lbnet.CreateFirewallRule("fw-proxy-access", "tcp", nil, []string{"10.1.2.128/25"}); err != nil {
		return err
	}

	mkvm := func(name, ip, test string) (*daisy.Instance, error) {
		inst := &daisy.Instance{}
		vm, err := t.CreateTestVMMultipleDisks([]*compute.Disk{{Name: name}}, inst)
		if err != nil {
			return nil, err
		}
		if err := vm.AddCustomNetwork(lbnet, lbsubnet); err != nil {
			return nil, err
		}
		if err := vm.SetPrivateIP(lbnet, ip); err != nil {
			return nil, err
		}
		vm.RunTests(test)
		return inst, nil
	}
	mkbackend := func(name, ip, test string) error { _, err := mkvm(name, ip, test); return err }
	mkclient := func(name, ip, test string) error {
		inst, err := mkvm(name, ip, test)
		if err != nil {
			return err
		}
		inst.Scopes = append(inst.Scopes, "https://www.googleapis.com/auth/cloud-platform")
		return nil
	}

	if err := mkbackend("l3backend1", l3backendVM1IP4addr, "TestL3Backend"); err != nil {
		return err
	}
	if err := mkbackend("l3backend2", l3backendVM2IP4addr, "TestL3Backend"); err != nil {
		return err
	}
	if err := mkclient("l3client", l3clientVMip4addr, "TestL3Client"); err != nil {
		return err
	}

	if err := mkbackend("l7backend1", l7backendVM1IP4addr, "TestL7Backend"); err != nil {
		return err
	}
	if err := mkbackend("l7backend2", l7backendVM2IP4addr, "TestL7Backend"); err != nil {
		return err
	}
	if err := mkclient("l7client", l7clientVMip4addr, "TestL7Client"); err != nil {
		return err
	}
	return nil
}
