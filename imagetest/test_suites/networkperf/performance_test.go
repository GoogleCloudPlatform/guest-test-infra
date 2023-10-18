//go:build cit
// +build cit

package networkperf

import (
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestNetworkPerformance(t *testing.T) {
	// Check performance of the driver.
	results, err := utils.GetMetadataGuestAttribute("testing/results")
	if err != nil {
		t.Fatalf("Error : Test results not found. %v", err)
	}

	// Get the performance target.
	expectedPerfString, err := utils.GetMetadataAttribute("expectedperf")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	expectedPerf, err := strconv.Atoi(expectedPerfString)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	expected := 0.85 * float64(expectedPerf)

	// Get machine type and network name for logging.
	machineType, err := utils.GetMetadata("machine-type")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	machineTypeSplit := strings.Split(machineType, "/")
	machineTypeName := machineTypeSplit[len(machineTypeSplit)-1]

	network, err := utils.GetMetadata("network-interfaces/0/network")
	if err != nil {
		t.Fatal(err)
	}
	networkSplit := strings.Split(network, "/")
	networkName := networkSplit[len(networkSplit)-1]

	// Find actual performance..
	var result_perf float64
	resultArray := strings.Split(results, " ")
	result_perf, err = strconv.ParseFloat(resultArray[5], 64)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Check if it matches the target.
	if result_perf < expected {
		t.Fatalf("Error: Did not meet performance expectation on machine type %s with network %s. Expected: %v Gbits/s, Actual: %v Gbits/s", machineTypeName, networkName, expected, result_perf)
	}
	t.Logf("Machine type: %v, Expected: %v Gbits/s, Actual: %v Gbits/s", machineTypeName, expected, result_perf)
}
