package metadata

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)


type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func TestTokenFetch(t *testing.T) {
	metadata, err := utils.GetMetadata("service-accounts/default/token")
	if err != nil {
		t.Fatalf("couldn't get token from metadata, err %v", err)
	}
	if err := json.Unmarshal([]byte(metadata), &Token{}); err != nil {
		t.Fatalf("token %s has incorrect format", metadata)
	}
}
