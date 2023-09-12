//go:build cit
// +build cit

package sql

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestSqlVersion(t *testing.T) {
	utils.WindowsOnly(t)

	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatal("Failed to get image metadata")
	}

	imageName, err := utils.ExtractBaseImageName(image)
	if err != nil {
		t.Fatal(err)
	}

	imageNameSplit := strings.Split(imageName, "-")
	sqlExpectedVer := imageNameSplit[1]
	sqlExpectedEdition := imageNameSplit[2]
	serverExpectedVer := imageNameSplit[4]

	command := fmt.Sprintf("Sqlcmd -Q \"select @@version\"")
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Unable to query SQL Server version: %v", err)
	}

	sqlOutput := strings.ToLower(strings.TrimSpace(output.Stdout))

	if !strings.Contains(sqlOutput, sqlExpectedEdition) {
		t.Fatalf("Installed SQL Server edition does not match image edition: %s not found in %s", sqlExpectedEdition, sqlOutput)
	}

	sqlVerString := "microsoft sql server " + sqlExpectedVer
	if !strings.Contains(sqlOutput, sqlVerString) {
		t.Fatalf("Installed SQL Server version does not match image version: %s not found in %s", sqlVerString, sqlOutput)
	}

	serverVerString := "on windows server " + serverExpectedVer
	if !strings.Contains(sqlOutput, serverVerString) {
		t.Fatalf("Installed Windows Server version does not match image version: %s not found in %s", serverVerString, sqlOutput)
	}
}

func TestPowerPlan(t *testing.T) {
	utils.WindowsOnly(t)

	command := fmt.Sprintf("powercfg /getactivescheme")
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Unable to query active power plan: %v", err)
	}

	activePlan := strings.ToLower(strings.TrimSpace(output.Stdout))
	expectedPlan := "high performance"
	if !strings.Contains(activePlan, expectedPlan) {
		t.Fatalf("Active power plan is not %s: got %s", expectedPlan, activePlan)
	}
}
func TestRemoteConnectivity(t *testing.T) {
	utils.WindowsOnly(t)

	testcommand := fmt.Sprintf("Get-LocalGroupMember -Group 'Administrators'")
	testoutput, err := utils.RunPowershellCmd(testcommand)
	fmt.Println(testoutput.Stdout)

	connectionCmd := `$SQLServer = (Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri 'http://metadata.google.internal/computeMetadata/v1/instance/attributes/sqltarget')
	$SQLDBName = 'master'
	$DBUser = 'sa'
	$DBPass = 'remoting@123'
	
	$SqlConnection = New-Object System.Data.SqlClient.SqlConnection
	$SqlConnection.ConnectionString = "Server = $SQLServer; Database = $SQLDBName; User ID = $DBUser; Password = $DBPass"
	
	$SqlCmd = New-Object System.Data.SqlClient.SqlCommand
	$SqlCmd.CommandText = 'SELECT * FROM information_schema.tables'
	$SqlCmd.Connection = $SqlConnection
	
	$SqlAdapter = New-Object System.Data.SqlClient.SqlDataAdapter
	$SqlAdapter.SelectCommand = $SqlCmd
	
	$DataSet = New-Object System.Data.DataSet
	$SqlAdapter.Fill($DataSet)
	$SqlConnection.Close()
	
	$DataSet | Out-File C:\remote_connection_output.txt`

	command := fmt.Sprintf(connectionCmd)
	fmt.Println("Running command:")
	fmt.Println(command)
	_, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Unable to query server database.")
	}

	bytes, err := ioutil.ReadFile("C:\\remote_connection_output.txt")
	if err != nil {
		t.Fatalf("Could not open test file for results.")
	}

	expectedData := "system.data.datarow"
	data := strings.TrimSpace(string(bytes))
	if !strings.Contains(strings.ToLower(data), expectedData) {
		t.Fatalf("Not found.")
	}
	fmt.Println("Test complete")
}
