//go:build cit
// +build cit

package gveperf

import (
	"runtime"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestNetworkPerformance(t *testing.T) {
	// Check performance of the driver.
	if runtime.GOOS != "windows" {
		results, err := utils.GetMetadataGuestAttribute("testing/results")
		if err != nil {
			t.Fatalf("Error : %v", err)
		}
		t.Logf(results)
	}
}
