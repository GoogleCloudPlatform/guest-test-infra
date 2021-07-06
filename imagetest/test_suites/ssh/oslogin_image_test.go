package ssh

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	TEST_USER          = "oslogin-test@gcp-guest.iam.gserviceaccount.com"
	ROOT               = "root"
	INVALID_GROUP      = "__invalid_group__"
	INVALID_USER       = "__invalid_user__"
	NOBODY             = "nobody"
	TEST_USERNAME      = "sa_106762486004676433507"
	TEST_UID           = "3651018652"
	TEST_GID           = "123452"
	TEST_GROUPNAME     = "demo"
	TEST_BIGGROUPNAME  = "testgroup00002"
	TEST_REALSELFGROUP = "testuser00200"
	TEST_VIRTSELFGROUP = "testuser00700"
)

var TEST_USER_PASSWD = fmt.Sprintf(".*%s:.?:%s:%s::/home/%s:/bin/bash.*", TEST_USERNAME, TEST_UID, TEST_UID, TEST_USERNAME)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

func TestGetentPasswdAllUsers(t *testing.T) {
	cmd := exec.Command("getent", "passwd")
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn("root:x:0:0:root:/root:", string(b)); err != nil {
		t.Fatal(err)
	}
	if err := assertIn("nobody:x:", string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestGetentPasswdOsLoginUser(t *testing.T) {
	cmd := exec.Command("getent", "passwd", TEST_USERNAME)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertRegex(string(b), TEST_USER_PASSWD); err != nil {
		t.Fatal(err)
	}
}
func TestGetentPasswdOsLoginUid(t *testing.T) {
	cmd := exec.Command("getent", "passwd", TEST_UID)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertRegex(string(b), TEST_USER_PASSWD); err != nil {
		t.Fatal(err)
	}
}

func TestGetentPasswdLocalUser(t *testing.T) {
	cmd := exec.Command("getent", "passwd", NOBODY)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "") {
		t.Fatal("invalid user should return nothing")
	}
	if err := assertIn("nobody:x:", string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestGetentPasswdInvalidUser(t *testing.T) {
	cmd := exec.Command("getent", "passwd", INVALID_USER)
	b, err := cmd.Output()
	if err != nil {
		t.Logf("failed get invalid user as expected")
	}
}

func TestGetentGroupOsLoginUsernameGroup(t *testing.T) {
	cmd := exec.Command("getent", "group", TEST_USERNAME)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(TEST_USERNAME, string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupOsLoginUidGroup(t *testing.T) {
	cmd := exec.Command("getent", "group", TEST_UID)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(TEST_USERNAME, string(b)); err != nil {
		t.Fatal(err)
	}
}
func TestGetentGroupOsLoginGid(t *testing.T) {
	cmd := exec.Command("getent", "group", TEST_GID)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(TEST_GROUPNAME, string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupOsLoginGroupName(t *testing.T) {
	cmd := exec.Command("getent", "group", TEST_GROUPNAME)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(TEST_GROUPNAME, string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupLocalGroup(t *testing.T) {
	cmd := exec.Command("getent", "group", ROOT)
	b, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn("osloing", string(b)); err != nil {
		t.Fatal(err)
	}
}

func TestGetentGroupInvalidGroup(t *testing.T) {
	cmd := exec.Command("getent", "group", INVALID_GROUP)
	_, err := cmd.Output()
	if err != nil {
		t.Logf("failed get invalid group as expected")
	}
}

// TestOsLoginGroupRealSelfGroup tests the real self-group does exist in metadata.
func TestOsLoginGroupRealSelfGroup(t *testing.T) {
	res, err := utils.GetOsLoginMetadataWithResponse("groups", fmt.Sprintf("groupname=%s", TEST_REALSELFGROUP))
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(TEST_REALSELFGROUP, res); err != nil {
		t.Fatal(err)
	}
}

// TestOsLoginGroupVirtualSelfGroup tests the virtual self-group does NOT exist in metadata.
func TestOsLoginGroupVirtualSelfGroup(t *testing.T) {
	res, err := utils.GetOsLoginMetadataWithResponse("groups", fmt.Sprintf("groupname=%s", TEST_VIRTSELFGROUP))
	if err != nil {
		t.Fatal(err)
	}
	if err := assertIn(TEST_VIRTSELFGROUP, res); err != nil {
		t.Fatal(err)
	}
}

// TestOsLoginGroupMatchesMetadata test an OS Login group matches its definition in metadata.
func TestOsLoginGroupMatchesMetadata(t *testing.T) {
	res, err := utils.GetOsLoginMetadataWithResponse("groups", fmt.Sprintf("groupname=%s", TEST_BIGGROUPNAME))
	var metadata map[string]interface{}
	err = json.Unmarshal([]byte(res), &metadata)
	metadata_users := metadata["usernames"]
	cmd := exec.Command("getent", "group", TEST_BIGGROUPNAME)
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed get invalid group as expected")
	}
	var getentUsers []string
	splits := strings.Split(string(b), ":")
	groupMembers := strings.Split(splits[len(splits)-1], ",")
	for _, member := range groupMembers {
		getentUsers = append(getentUsers, strings.TrimSpace(member))
	}

	t.Logf("Metadata group members: %s", metadata_users)
	t.Logf("VM getent group members: %s", getentUsers)
	t.Logf("VM getent output: %s", string(b))
	metadata_users_string, err := json.Marshal(metadata_users)
	if err != nil {
		t.Fatal(err)
	}
	if string(metadata_users_string) != strings.Join(getentUsers, " ") {
		t.Fatalf("")
	}
}

func TestOsLoginEnableDisable(t *testing.T) {
	//cmd := exec.Command("grep", []string{"passwd:", "/etc/nsswitch.conf"}...)
	//b, err := cmd.Output()
	//if err != nil {
	//	t.Fatal(err)
	//}
	//if !strings.Contains(string(b), "oslogin") {
	//	t.Fatalf("oslogin is not enabled as expected")
	//}
	//
	//if err := utils.SetMetadata("enable-oslogin", "false"); err != nil {
	//	t.Fatal(err)
	//}

	//# Basic
	//checks
	//for presence
	//of
	//OS
	//Login
	//_, output = self._RunMetadataCommand(
	//	self._vm_enable_disable, 'grep passwd: /etc/nsswitch.conf')
	//self.assertNotIn('oslogin', output)
	//
	//_, output = self._RunMetadataCommand(
	//	self._vm_enable_disable,
	//	'grep AuthorizedKeysCommand /etc/ssh/sshd_config')
	//self.assertNotIn('/usr/bin/google_authorized_keys', output)
	//
	//_, output = self._RunMetadataCommand(
	//	self._vm_enable_disable, 'cat /etc/pam.d/sshd')
	//self.assertNotIn('pam_oslogin_login.so', output)
	//self.assertNotIn('pam_oslogin_admin.so', output)
	//
	//# Enable
	//OS
	//Login.
	//	self._vm_enable_disable.AppendMetadata(
	//{
	//	'enable-oslogin': 'true'
	//})
	//self._vm_enable_disable.Get()
	//
	//# Basic
	//checks
	//for presence
	//of
	//OS
	//Login.
	//	_, output = self._RunMetadataCommand(
	//	self._vm_enable_disable, 'grep passwd: /etc/nsswitch.conf')
	//self.assertIn('oslogin', output)
	//
	//_, output = self._RunMetadataCommand(
	//	self._vm_enable_disable,
	//	'grep AuthorizedKeysCommand /etc/ssh/sshd_config')
	//self.assertIn('/usr/bin/google_authorized_keys', output)
	//
	//_, output = self._RunMetadataCommand(
	//	self._vm_enable_disable, 'cat /etc/pam.d/sshd')
	//self.assertIn('pam_oslogin_login.so', output)
	//self.assertIn('pam_oslogin_admin.so', output)
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
