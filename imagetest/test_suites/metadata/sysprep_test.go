//go:build cit
// +build cit

package metadata

import (
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestSysprepSpecialize(t *testing.T) {
	utils.WindowsOnly(t)
	result, err := utils.GetMetadata(utils.Context(t), "instance", "guest-attributes", "testing", "result")
	if err != nil {
		t.Fatalf("failed to read startup script result key: %v", err)
	}
	if result != expectedStartupContent {
		t.Fatalf(`sysprep-specialize script output expected "%s", got "%s".`, expectedStartupContent, result)
	}
}
