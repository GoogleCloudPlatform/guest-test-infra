package oslogin

import (
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/utils/metadata"
)

func TestXxx(t *testing.T) {
	fmt.Println("oslogin.TestXxx")
	metadata, code, err := metadata.GetMetadata("attributes/hostname")
	if err == nil && code == 200 {
		fmt.Printf("hostname: %s\n", metadata)
	}
}
