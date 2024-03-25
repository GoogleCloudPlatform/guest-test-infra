// Copyright 2023 Google LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     https://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build cit
// +build cit

package mdsmtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func checkMTLSAvailable(t *testing.T) {
	t.Helper()
	ctx := utils.Context(t)
	if _, err := utils.GetMetadata(ctx, "instance", "credentials", "certs"); err != nil {
		t.Skip("MTLs certs are not available from the MDS")
	}
}

// checkCredsPresent checks mTLS creds exist on Linux based OSs.
// metadata-script-runner has service dependency and is guaranteed to run after guest-agent.
func checkCredsPresent(t *testing.T) {
	t.Helper()

	credsDir := "/run/google-mds-mtls"
	creds := []string{filepath.Join(credsDir, "root.crt"), filepath.Join(credsDir, "client.key")}

	for _, f := range creds {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("os.Stat(%s) failed with error: %v, mTLS creds expected to be present at %q", f, err, f)
		}
	}
}

// checkCredsPresentWindows checks if mTLS creds exist on Windows systems.
// Unlike Linux metadata-script-runner is not guaranteed to run after guest-agent and implements
// a retry logic to avoid timing issues.
func checkCredsPresentWindows(t *testing.T) {
	t.Helper()

	credsDir := filepath.Join(os.Getenv("ProgramData"), "Google", "Compute Engine")
	creds := []string{filepath.Join(credsDir, "mds-mtls-root.crt"), filepath.Join(credsDir, "mds-mtls-client.key"), filepath.Join(credsDir, "mds-mtls-client.key.pfx")}

	var lastErrors []string

	// Try to test every 10 sec for max 2 minutes.
	for i := 1; i <= 12; i++ {
		// Reset the list before every retry.
		lastErrors = nil
		for _, f := range creds {
			if _, err := os.Stat(f); err != nil {
				lastErrors = append(lastErrors, fmt.Sprintf("os.Stat(%s) failed with error: %v, mTLS creds expected to be present at %q", f, err, f))
			}
		}

		if len(lastErrors) == 0 {
			break
		}
		time.Sleep(10 * time.Second)
	}

	if len(lastErrors) != 0 {
		t.Fatalf("Exhausted all retries, failed to check mTLS credentials with error: %v", lastErrors)
	}
}

func TestMTLSCredsExists(t *testing.T) {
	checkMTLSAvailable(t)
	ctx := utils.Context(t)
	var rootKeyFile, clientKeyFile string
	if utils.IsWindows() {
		rootKeyFile = filepath.Join(os.Getenv("ProgramData"), "Google", "Compute Engine", "mds-mtls-root.crt")
		clientKeyFile = filepath.Join(os.Getenv("ProgramData"), "Google", "Compute Engine", "mds-mtls-client.crt")
		checkCredsPresentWindows(t)
	} else {
		rootKeyFile = filepath.Join("run", "google-mds-mtls", "root.crt")
		clientKeyFile = filepath.Join("run", "google-mds-mtls", "client.crt")
		checkCredsPresent(t)
	}
	certPair, err := tls.LoadX509KeyPair(rootKeyFile, clientKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	rootKey, err := ioutil.ReadFile(rootKeyFile)
	if err != nil {
		t.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(rootKey)
	client := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
				Certificates: []tls.Certificate{certPair},
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://169.254.169.254/computeMetadata/v1/instance/hostname", nil)
	if err != nil {
		t.Fatalf("could not make http request: %v", err)
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if resp.TLS == nil {
		t.Errorf("Metadata response was sent unencrypted")
	}
}

func TestMTLSJobScheduled(t *testing.T) {
	checkMTLSAvailable(t)
	ctx := utils.Context(t)
	var cmd *exec.Cmd
	if utils.IsWindows() {
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NonInteractive", "Get-WinEvent", "-Providername", "GCEGuestAgent")
	} else {
		cmd = exec.CommandContext(ctx, "journalctl", "-o", "cat", "-eu", "google-guest-agent")
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("could not get agent output: %v", err)
	}
	if !strings.Contains(string(out), "Successfully scheduled job MTLS_MDS_Credential_Boostrapper") {
		t.Errorf("guest agent has not scheduled the mtls credential bootstrapper")
	}
}
