package ssh

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"golang.org/x/crypto/ssh"
)

const (
	//TestUser          = "oslogin-test@gcp-guest.iam.gserviceaccount.com"
	root              = "root"
	invalidGroup      = "__invalid_group__"
	invalidUser       = "__invalid_user__"
	nobody            = "nobody"
	testUsername      = "sa_106762486004676433507"
	testUID           = "3651018652"
	testGID           = "123452"
	testGroupname     = "demo"
	testBiggroupname  = "testgroup00002"
	testRealSelfgroup = "testuser00200"
	testVirtSelfgroup = "testuser00700"
)

var testUserPasswd = fmt.Sprintf(".*%s:.?:%s:%s::/home/%s:/bin/bash.*", testUsername, testUID, testUID, testUsername)

func TestGetentPasswdAllUsers(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, "getent passwd")
	if err != nil {
		t.Fatalf("failed to run cmd on remote host, err %v", err)
	}
	if err := assertIn("root:x:0:0:root:/root:", out); err != nil {
		t.Fatal(err)
	}
	if err := assertIn("nobody:x:", out); err != nil {
		t.Fatal(err)
	}
}

func TestGetentPasswdOsLoginUser(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent passwd %s", testUsername))
	if err != nil {
		t.Fatalf("failed to run cmd on remote host, err %v", err)
	}
	if err := assertRegex(out, testUserPasswd); err != nil {
		t.Fatal(err)
	}
}
func TestGetentPasswdOsLoginUid(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent passwd %s", testUID))
	if err != nil {
		t.Fatalf("failed to run cmd on remote host, err %v", err)
	}
	if err := assertRegex(out, testUserPasswd); err != nil {
		t.Fatal(err)
	}
}

func TestGetentPasswdLocalUser(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent passwd %s", nobody))

	if !strings.Contains(out, "") {
		t.Fatal("invalid user should return nothing")
	}
	if err := assertIn("nobody:x:", out); err != nil {
		t.Fatal(err)
	}
}

func TestGetentPasswdInvalidUser(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCmdOnRemote(client, fmt.Sprintf("getent passwd %s", invalidUser))

	if err != nil {
		t.Logf("failed get invalid user as expected")
	}
}

func TestGetentGroupOsLoginUsernameGroup(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent group %s", testUsername))

	if err := assertIn(testUsername, out); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupOsLoginUidGroup(t *testing.T) {
	cmd := exec.Command("getent", "group", testUID)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(testUsername, string(b)); err != nil {
		t.Fatal(err)
	}
}
func TestGetentGroupOsLoginGid(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent group %s", testGID))
	if err := assertIn(testGroupname, out); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupOsLoginGroupName(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent group %s", testGroupname))
	if err := assertIn(testGroupname, out); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupLocalGroup(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := runCmdOnRemote(client, fmt.Sprintf("getent group %s", root))
	if err := assertIn("oslogin", out); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupInvalidGroup(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runCmdOnRemote(client, fmt.Sprintf("getent group %s", invalidGroup))
	if err != nil {
		t.Logf("failed get invalid group as expected")
	}
}

// TestOsLoginGroupRealSelfGroup tests the real self-group does exist in metadata.
func TestOsLoginGroupRealSelfGroup(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	queryURL := getOsLoginMetadataQueryPath("groups", map[string]string{"groupname": testRealSelfgroup})
	out, err := runCmdOnRemote(client, fmt.Sprintf(`curl %s -H "Metadata-Flavor: Google"`, queryURL))
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(testRealSelfgroup, out); err != nil {
		t.Fatal(err)
	}
}

// TestOsLoginGroupVirtualSelfGroup tests the virtual self-group does NOT exist in metadata.
func TestOsLoginGroupVirtualSelfGroup(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	queryURL := getOsLoginMetadataQueryPath("groups", map[string]string{"groupname": testVirtSelfgroup})
	out, err := runCmdOnRemote(client, fmt.Sprintf(`curl %s -H "Metadata-Flavor: Google"`, queryURL))
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(testVirtSelfgroup, out); err != nil {
		t.Fatal(err)
	}
}

// TestOsLoginGroupMatchesMetadata test an OS Login group matches its definition in metadata.
func TestOsLoginGroupMatchesMetadata(t *testing.T) {
	client, err := createClient("vm3", osloginUser)
	if err != nil {
		t.Fatal(err)
	}
	queryURL := getOsLoginMetadataQueryPath("groups", map[string]string{"groupname": testBiggroupname})
	out, err := runCmdOnRemote(client, fmt.Sprintf(`curl %s -H "Metadata-Flavor: Google"`, queryURL))
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]interface{}
	err = json.Unmarshal([]byte(out), &metadata)
	metadataUsers := metadata["usernames"]

	out, err = runCmdOnRemote(client, fmt.Sprintf("getent group %s", testBiggroupname))
	if err != nil {
		t.Fatalf("failed get invalid group as expected")
	}
	var getentUsers []string
	splits := strings.Split(out, ":")
	groupMembers := strings.Split(splits[len(splits)-1], ",")
	for _, member := range groupMembers {
		getentUsers = append(getentUsers, strings.TrimSpace(member))
	}

	t.Logf("Metadata group members: %s", metadataUsers)
	t.Logf("VM getent group members: %s", getentUsers)
	t.Logf("VM getent output: %s", out)
	metadataUsersString, err := json.Marshal(metadataUsers)
	if err != nil {
		t.Fatal(err)
	}
	if string(metadataUsersString) != strings.Join(getentUsers, " ") {
		t.Fatalf("")
	}
}

// Private method

func createClient(vm, user string) (*ssh.Client, error) {
	vmname, err := utils.GetRealVMName(vm)
	if err != nil {
		return nil, fmt.Errorf("failed to get real vm name: %v", err)
	}
	pembytes, err := utils.DownloadPrivateKey(user)
	if err != nil {
		return nil, fmt.Errorf("failed to download private key: %v", err)
	}
	client, err := utils.CreateClient(user, fmt.Sprintf("%s:22", vmname), pembytes)
	if err != nil {
		return nil, fmt.Errorf("failed create ssh client for user %s on vm %s, err %v", user, vmname, err)
	}
	return client, nil
}

func runCmdOnRemote(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	b, err := session.Output(cmd)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func assertIn(search, target string) error {
	if !strings.Contains(target, search) {
		return fmt.Errorf("%s not found in %s", search, target)
	}
	return nil
}

func assertRegex(search, regex string) error {
	re, err := regexp.Compile(regex)
	if err != nil {
		return err
	}
	if re.FindString(search) == "" {
		return fmt.Errorf("regex %s not match in %s", regex, search)
	}
	return nil
}

// getOsLoginMetadataQueryPath construct an oslogin request url path.
func getOsLoginMetadataQueryPath(path string, urlParams map[string]string) string {
	params := url.Values{}
	for key, value := range urlParams {
		params.Add(key, value)
	}
	osLoginPath := fmt.Sprintf("oslogin/%s", path)
	if urlParams != nil {
		osLoginPath += fmt.Sprintf("?%s", params.Encode())
	}
	return osLoginPath
}
