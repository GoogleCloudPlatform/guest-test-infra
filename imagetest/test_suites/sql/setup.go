package sql

import (
	"embed"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// Name is the name of the test package. It must match the directory name.
var Name = "sql"

// InstanceConfig for setting up test VMs.
type InstanceConfig struct {
	name string
	ip   string
}

var serverConfig = InstanceConfig{name: "server-vm", ip: "192.168.0.10"}
var clientConfig = InstanceConfig{name: "client-vm", ip: "192.168.0.11"}

//go:embed *
var scripts embed.FS

const (
	serverStartupScriptURL = "startupscripts/remote_auth_server_setup.ps1"
	clientStartupScriptURL = "startupscripts/remote_auth_client_setup.ps1"
)

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	if utils.HasFeature(t.Image, "WINDOWS") {
		defaultNetwork, err := t.CreateNetwork("default-network", false)
		if err != nil {
			return err
		}
		defaultSubnetwork, err := defaultNetwork.CreateSubnetwork("default-subnetwork", "192.168.0.0/24")
		if err != nil {
			return err
		}
		if err := defaultNetwork.CreateFirewallRule("allow-sql-tcp", "tcp", []string{"135", "1433", "1434", "4022", "5022"}, []string{"192.168.0.0/24"}); err != nil {
			return err
		}
		if err := defaultNetwork.CreateFirewallRule("allow-sql-udp", "udp", []string{"1434"}, []string{"192.168.0.0/24"}); err != nil {
			return err
		}

		// Get the startup scripts as byte arrays.
		serverStartupByteArr, err := scripts.ReadFile(serverStartupScriptURL)
		if err != nil {
			return err
		}
		clientStartupByteArr, err := scripts.ReadFile(clientStartupScriptURL)
		if err != nil {
			return err
		}
		serverStartup := string(serverStartupByteArr)
		clientStartup := string(clientStartupByteArr)

		serverVM, err := t.CreateTestVM(serverConfig.name)
		if err != nil {
			return err
		}
		if err := serverVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
			return err
		}
		if err := serverVM.SetPrivateIP(defaultNetwork, serverConfig.ip); err != nil {
			return err
		}

		clientVM, err := t.CreateTestVM(clientConfig.name)
		if err != nil {
			return err
		}
		if err := clientVM.AddCustomNetwork(defaultNetwork, defaultSubnetwork); err != nil {
			return err
		}
		if err := clientVM.SetPrivateIP(defaultNetwork, clientConfig.ip); err != nil {
			return err
		}
		clientVM.AddMetadata("enable-guest-attributes", "TRUE")
		clientVM.AddMetadata("sqltarget", serverConfig.ip)

		serverVM.AddMetadata("windows-startup-script-ps1", serverStartup)
		clientVM.AddMetadata("windows-startup-script-ps1", clientStartup)

		vm1, err := t.CreateTestVM("vm1")
		if err != nil {
			return err
		}
		vm1.RunTests("TestSqlVersion|TestPowerPlan")
		clientVM.RunTests("TestRemoteConnectivity")
		serverVM.RunTests("TestPowerPlan")
	}
	return nil
}
