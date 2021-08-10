//go:build cit
// +build cit

package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const metadataURLIPPrefix = "http://169.254.169.254/computeMetadata/v1/instance/"

type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// TestTokenFetch test service-accounts token could be retrieved from metadata.
func TestTokenFetch(t *testing.T) {
	metadata, err := utils.GetMetadata("service-accounts/default/token")
	if err != nil {
		t.Fatalf("couldn't get token from metadata, err % v", err)
	}
	if err := json.Unmarshal([]byte(metadata), &Token{}); err != nil {
		t.Fatalf("token %s has incorrect format", metadata)
	}
}

// TestMetaDataResponseHeaders verify that HTTP response headers do not include confidential data.
func TestMetaDataResponseHeaders(t *testing.T) {
	resp, err := utils.GetMetadataHTTPResponse("id")
	if err != nil {
		t.Fatalf("couldn't get id from metadata, err % v", err)
	}
	for key, values := range resp.Header {
		if key != "Metadata-Flavor" {
			for _, v := range values {
				if strings.Contains(strings.ToLower(v), "google") {
					t.Fatal("unexpected Google header exists in metadata response")
				}
			}
		}
	}
}

// TestGetMetaDataUsingIP test that metadata can be retrieved by IP
func TestGetMetaDataUsingIP(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", metadataURLIPPrefix, ""), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("http response code is %v", resp.StatusCode)
	}
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}
