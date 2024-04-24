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
	"strings"
	"testing"
	"time"

	daisyCompute "github.com/GoogleCloudPlatform/compute-daisy/compute"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

const user = "windowsuser5"

type windowsKeyJSON struct {
	ExpireOn string
	Exponent string
	Modulus  string
	UserName string
}

// unlike utils.GetMetadata(), this gets the full metadata object for the instance rather than the metadata stored at a single url path
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

func getEncryptedPassword(client daisyCompute.Client, mod, instanceName, projectID, zone string) (string, error) {
	out, err := client.GetSerialPortOutput(projectID, zone, instanceName, 4, 0)
	if err != nil {
		return "", fmt.Errorf("could not get serial output: err %v", err)
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

func resetPassword(client daisyCompute.Client, t *testing.T) (string, error) {
	ctx := utils.Context(t)
	instanceName, err := utils.GetInstanceName(ctx)
	if err != nil {
		return "", fmt.Errorf("could not get instsance name: err %v", err)
	}
	projectID, zone, err := utils.GetProjectZone(ctx)
	if err != nil {
		return "", fmt.Errorf("could not project or zone: err %v", err)
	}
	md, err := getInstanceMetadata(client, instanceName, zone, projectID)
	if err != nil {
		return "", fmt.Errorf("error getting instance metadata: instance %s, zone %s, project %s, err %v", instanceName, zone, projectID, err)
	}
	t.Log("Generating public/private key pair")
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

	if err := client.SetInstanceMetadata(projectID, zone, instanceName, md); err != nil {
		return "", err
	}
	t.Logf("Set new 'windows-keys' metadata to %s", winKeys)

	t.Log("Fetching encrypted password")
	var attempts int
	var ep string
	for {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context expired before successfully fetching encrypted password: %v", err)
		}
		time.Sleep(time.Minute)
		ep, err = getEncryptedPassword(client, winKey.Modulus, instanceName, projectID, zone)
		if err == nil {
			break
		}
		if attempts > 5 {
			return "", err
		}
		attempts++
	}

	t.Log("Decrypting password")
	return decryptPassword(key, ep)
}

// Verifies that a powershell command ran with no errors and exited with an exit code of 0.
// If a username or password was invalid, this should result in a testing error.
// Returns the standard output in case it needs to be used later.
func verifyPowershellCmd(t *testing.T, cmd string) string {
	procStatus, err := utils.RunPowershellCmd(cmd)
	if err != nil {
		t.Fatalf("cmd %s failed: stdout %s, stderr %v, error %v", cmd, procStatus.Stdout, procStatus.Stderr, err)
	}

	stdout := procStatus.Stdout
	if procStatus.Exitcode != 0 {
		t.Fatalf("cmd %s failed with exitcode %d, stdout %s and stderr %s", cmd, procStatus.Exitcode, stdout, procStatus.Stderr)
	}
	return stdout
}

func TestWindowsPasswordReset(t *testing.T) {
	utils.WindowsOnly(t)
	initpwd := "gyug3q445m0!"
	createUserCmd := fmt.Sprintf("net user %s %s /add", user, initpwd)
	verifyPowershellCmd(t, createUserCmd)
	ctx := utils.Context(t)
	client, err := daisyCompute.NewClient(ctx)
	if err != nil {
		t.Fatalf("Error creating compute service: %v", err)
	}

	t.Logf("Resetting password on current instance for user %q\n", user)
	decryptedPassword, err := resetPassword(client, t)
	if err != nil {
		t.Fatalf("reset password failed: error %v", err)
	}
	t.Logf("- Username: %s\n- Password: %s\n", user, decryptedPassword)
	// wait for guest agent to update, since it can take up to a minute
	time.Sleep(time.Minute)
	getUsersCmd := "Get-CIMInstance Win32_UserAccount | ForEach-Object { Write-Output $_.Name}"
	userList := verifyPowershellCmd(t, getUsersCmd)
	t.Logf("expected user %s in userlist %s", user, userList)
	if !strings.Contains(userList, user) {
		t.Fatalf("user %s not found in userlist: %s", user, userList)
	}
	verificationCmd := fmt.Sprintf("Start-Process -Credential (New-Object System.Management.Automation.PSCredential(\"%s\", ('%s' | ConvertTo-SecureString -AsPlainText -Force))) -WorkingDirectory C:\\Windows\\System32 -FilePath cmd.exe", user, decryptedPassword)
	// The process "Credential" in powershell does not print anything on success
	verifyPowershellCmd(t, verificationCmd)
}
