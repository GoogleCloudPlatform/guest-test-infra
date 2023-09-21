//go:build cit
// +build cit

package networkperf

import (
	"encoding/json"
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
	var targetMap map[string]int
	targetMapString, err := utils.GetMetadataAttribute("perfmap")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if err := json.Unmarshal([]byte(targetMapString), &targetMap); err != nil {
		t.Fatalf("Error: %v", err)
	}
	machineType, err := utils.GetMetadata("machine-type")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	machineTypeSplit := strings.Split(machineType, "/")
	machineTypeName := machineTypeSplit[len(machineTypeSplit)-1]
	target, found := targetMap[machineTypeName]
	if !found {
		t.Logf("%v not supported in this test", machineTypeName)
		return
	}
	expected := 0.85 * float64(target)

	// Find actual performance..
	var result_perf float64
	resultArray := strings.Split(results, " ")
	result_perf, err = strconv.ParseFloat(resultArray[5], 64)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Check if it matches the target.
	if result_perf < expected {
		t.Fatalf("Error: Did not meet performance expectation. Expected: %v Gbits/s, Actual: %v Gbits/s", expected, result_perf)
	}
	t.Logf("Machine type: %v, Expected: %v Gbits/s, Actual: %v Gbits/s", machineTypeName, expected, result_perf)
}
