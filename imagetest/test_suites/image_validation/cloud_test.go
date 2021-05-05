package imagevalidation

import (
	"strconv"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestSystemClock(t *testing.T) {
	driftToken, err := utils.GetMetadata("virtual-clock/drift-token")
	if err != nil {
		t.Fatalf("failed getting drift token from metadata")
	}
	value, err := strconv.Atoi(driftToken)
	if err != nil {
		t.Fatal("failed convert to integer")
	}
	if value != 0 {
		t.Fatalf("driftToken is %d which is not expected", value)
	}
}
