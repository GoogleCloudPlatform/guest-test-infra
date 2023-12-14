//  Copyright 2018 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package guestagent

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	daisyCompute "github.com/GoogleCloudPlatform/compute-daisy/compute"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

const user = "windowstester5"

type windowsKeyJSON struct {
	ExpireOn string
	Exponent string
	Modulus  string
	UserName string
}

func getInstanceMetadata(client daisyCompute.Client, instance, zone, project string) (*compute.Metadata, error) {
	ins, err := client.GetInstance(project, zone, instance)
	if err != nil {
		return nil, fmt.Errorf("error getting instance: %v", err)
	}

	return ins.Metadata, nil
}

func generateKey(priv *rsa.PublicKey) (*windowsKeyJSON, error) {
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, uint32(priv.E))

	return &windowsKeyJSON{
		ExpireOn: time.Now().Add(5 * time.Minute).Format(time.RFC3339),
		// This is different than what the other tools produce,
		// AQAB vs AQABAA==, both are decoded as 65537.
		Exponent: base64.StdEncoding.EncodeToString(bs),
		Modulus:  base64.StdEncoding.EncodeToString(priv.N.Bytes()),
		UserName: user,
	}, nil
}

type credsJSON struct {
	ErrorMessage      string `json:"errorMessage,omitempty"`
	EncryptedPassword string `json:"encryptedPassword,omitempty"`
	Modulus           string `json:"modulus,omitempty"`
}

// gets the instance name, zone, and project id name as strings
func getProjectZoneAndInstanceName() (string, string, string, error) {
	var fqdnString string
	if runtime.GOOS == "windows" {
		procStatus, err := utils.RunPowershellCmd("Invoke-RestMethod -Headers @{'Metadata-Flavor' = 'Google'} -Uri \"http://metadata.google.internal/computeMetadata/v1/instance/hostname\"")
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get project, zone, or instance on windows: %v", err)
		}
		fqdnString = strings.TrimSpace(procStatus.Stdout)
	} else {
		fqdnBytes, err := exec.Command("curl", "http://metadata.google.internal/computeMetadata/v1/instance/hostname", "-H", "Metadata-Flavor: Google").Output()
		fqdnString = strings.TrimSpace(string(fqdnBytes))
		if err != nil {
			return "", "", "", fmt.Errorf("failed to get project or zone on linux: %v", err)
		}
	}
	// See the docs for the FQDN: https://cloud.google.com/compute/docs/internal-dns#instance-fully-qualified-domain-names
	// the returned string should be in the format VM_NAME.ZONE.c.PROJECT_ID.internal
	fqdnTokens := strings.Split(string(fqdnString), ".")
	if len(fqdnTokens) != 5 || strings.ToLower(fqdnTokens[4]) != "internal" || strings.ToLower(fqdnTokens[2]) != "c" {
		return "", "", "", fmt.Errorf("returned string for vm metata was the wrong format: got %s", fqdnString)
	}

	// return format is (projectNumber, instanceZone, nil)
	return fqdnTokens[0], fqdnTokens[1], fqdnTokens[3], nil
}

func getEncryptedPassword(client daisyCompute.Client, mod string) (string, error) {
	instanceName, zone, projectId, err := getProjectZoneAndInstanceName()
	if err != nil {
		return "", fmt.Errorf("could not project, zone or instance name: err %v", err)
	}
	out, err := client.GetSerialPortOutput(projectId, zone, instanceName, 4, 0)
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(out.Contents, "\n") {
		var creds credsJSON
		if err := json.Unmarshal([]byte(line), &creds); err != nil {
			continue
		}
		if creds.Modulus == mod {
			if creds.ErrorMessage != "" {
				return "", fmt.Errorf("error from agent: %s", creds.ErrorMessage)
			}
			return creds.EncryptedPassword, nil
		}
	}
	return "", fmt.Errorf("password not found in serial output: %s", out.Contents)
}

func decryptPassword(priv *rsa.PrivateKey, ep string) (string, error) {
	bp, err := base64.StdEncoding.DecodeString(ep)
	if err != nil {
		return "", fmt.Errorf("error decoding password: %v", err)
	}
	pwd, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, priv, bp, nil)
	if err != nil {
		return "", fmt.Errorf("error decrypting password: %v", err)
	}
	return string(pwd), nil
}

func createNewWindowsUser() {
	username := "windowsuser5"
	password := "gyug3q445m0!"

	createUserCmd := fmt.Sprintf("net user %s %s /add", username, password)
	verificationCmd := fmt.Sprintf("Start-Process -Credential (New-Object System.Management.Automation.PSCredential(\"%s\", (\"%s\" | ConvertTo-SecureString -AsPlainText -Force))) -WorkingDirectory C:\\Windows\\System32 -FilePath cmd.exe", username, password)

	utils.RunPowershellCmd(createUserCmd)
	utils.RunPowershellCmd(verificationCmd)
}

func resetPassword(client daisyCompute.Client, t *testing.T) (string, error) {
	instanceName, zone, projectId, err := getProjectZoneAndInstanceName()
	if err != nil {
		return "", fmt.Errorf("could not project, zone or instance name: err %v", err)
	}
	md, err := getInstanceMetadata(client, instanceName, zone, projectId)
	if err != nil {
		return "", fmt.Errorf("error getting instance metadata: instance %s, zone %s, project %s, err %v", instanceName, zone, projectId, err)
	}
	fmt.Println("Generating public/private key pair")
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", err
	}

	winKey, err := generateKey(&key.PublicKey)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(winKey)
	if err != nil {
		return "", err
	}

	winKeys := string(data)
	var found bool
	for _, mdi := range md.Items {
		if mdi.Key == "windows-keys" {
			val := fmt.Sprintf("%s\n%s", *mdi.Value, winKeys)
			mdi.Value = &val
			found = true
			break
		}
	}
	if !found {
		md.Items = append(md.Items, &compute.MetadataItems{Key: "windows-keys", Value: &winKeys})
	}

	fmt.Println("Setting new 'windows-keys' metadata")
	if err := client.SetInstanceMetadata(projectId, zone, instanceName, md); err != nil {
		return "", err
	}
	t.Logf("Setting new 'windows-keys' metadata to %s", winKeys)

	fmt.Println("Fetching encrypted password")
	var trys int
	var ep string
	for {
		time.Sleep(1 * time.Second)
		ep, err = getEncryptedPassword(client, winKey.Modulus)
		if err == nil {
			break
		}
		if trys > 10 {
			return "", err
		}
		trys++
	}

	fmt.Println("Decrypting password")
	return decryptPassword(key, ep)
}

func TestWindowsPasswordReset(t *testing.T) {
	createNewWindowsUser()
	ctx := utils.Context(t)
	client, err := daisyCompute.NewClient(ctx)
	if err != nil {
		t.Fatalf("Error creating compute service: %v", err)
	}

	fmt.Printf("Resetting password on current instance for user %q\n", user)
	pw, err := resetPassword(client, t)
	if err != nil {
		t.Fatalf("reset password failed: error %v", err)
	}
	fmt.Printf("- Username: %s\n- Password: %s\n", user, pw)
}
