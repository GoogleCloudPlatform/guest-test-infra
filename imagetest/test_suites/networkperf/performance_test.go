//go:build cit
// +build cit

package networkperf

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestNetworkPerformance(t *testing.T) {
	// Check performance of the driver.
	var results string
	var err error
	for i := 0; i < 3; i++ {
		time.Sleep(time.Duration(i) * time.Second)
		results, err = utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "results")
		if err == nil {
			break
		}
		if i == 2 {
			t.Fatalf("Error : Test results not found. %v", err)
		}
	}

	// Get the performance target.
	expectedPerfString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "expectedperf")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	expectedPerf, err := strconv.Atoi(expectedPerfString)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	expected := 0.85 * float64(expectedPerf)

	// Get machine type and network name for logging.
	machineType, err := utils.GetMetadata(utils.Context(t), "instance", "machine-type")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	machineTypeSplit := strings.Split(machineType, "/")
	machineTypeName := machineTypeSplit[len(machineTypeSplit)-1]

	network, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", "network-tier")
	if err != nil {
		t.Fatal(err)
	}

	// Find actual performance.
	resultsArray := strings.Split(results, " ")
	result_perf, err := strconv.ParseFloat(resultsArray[5], 64)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Check the units.
	units := resultsArray[6]
	if !strings.HasPrefix(units, "G") { // If the units aren't in Gbits/s, we automatically fail.
		t.Fatalf("Error: Did not meet performance expectation on machine type %s with network %s. Expected: %v Gbits/s, Actual: %v %s", machineTypeName, network, expected, result_perf, units)
	}

	// Check if it matches the target.
	if result_perf < expected {
		t.Fatalf("Error: Did not meet performance expectation on machine type %s with network %s. Expected: %v Gbits/s, Actual: %v Gbits/s", machineTypeName, network, expected, result_perf)
	}
	t.Logf("Machine type: %v, Expected: %v Gbits/s, Actual: %v Gbits/s", machineTypeName, expected, result_perf)
}
