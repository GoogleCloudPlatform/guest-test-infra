package oslogin

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/utils"
)

func TestXxx(t *testing.T) {
	fmt.Println("oslogin.TestXxx")
	metadata, err := utils.GetMetadataAttribute("hostname")
	if err == nil {
		fmt.Printf("hostname: %s\n", metadata)
	} else {
		t.Errorf("couldn't determine hostname from metadata")
	}
}
