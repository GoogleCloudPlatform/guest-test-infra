//go:build cit
// +build cit

package sql

import (
	"fmt"
	"strconv"
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
	$result = $SqlAdapter.Fill($DataSet)
	$SqlConnection.Close()
	
	Invoke-RestMethod -Method Put -Body $result -Headers @{'Metadata-Flavor' = 'Google'} -Uri 'http://metadata.google.internal/computeMetadata/v1/instance/guest-attributes/testing/result' -ContentType "application/json; charset=utf-8" -UseBasicParsing`

	command := fmt.Sprintf(connectionCmd)
	output, err := utils.RunPowershellCmd(command)
	if err != nil {
		t.Fatalf("Unable to query server database: %v", err)
	}

	data, err := utils.GetMetadataGuestAttribute("testing/result")
	if err != nil {
		t.Fatalf("Failed to get test result from metadata: %v", err)
	}

	if strconv.ParseInt(data, 10, 32) > 0 {
		t.Fatalf("Test failed: query returned %s rows, expected > 0.", data)
	}
}
