package hostkey

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var (
	runtest = flag.Bool("runtest", false, "really run the test")
)

func getHostKeysFromDisk() (map[string]string, error) {
	bytes, err := ioutil.ReadFile("/etc/ssh/ssh_host_*_key.pub")
	if err != nil {
		return nil, err
	}
	hostkeyLines := strings.Split(strings.TrimSpace(string(bytes)), "\n")

	var hostkeyMap = make(map[string]string)
	for _, hostkey := range hostkeyLines {
		keyType := strings.Split(hostkey, " ")[0]
		keyValue := strings.Split(hostkey, " ")[1]
		hostkeyMap[keyType] = keyValue
	}

	return hostkeyMap, nil
}

func TestMain(m *testing.M) {
	flag.Parse()
	if *runtest {
		os.Exit(m.Run())
	} else {
		os.Exit(0)
	}
}

// TestMatchingKeysInGuestAttributes validate that host keys in guest attributes match those on disk.
func TestMatchingKeysInGuestAttributes(t *testing.T) {
	diskEntries, err := getHostKeysFromDisk()
	if err != nil {
		t.Fatalf("failed to get host key from disk %v", err)
	}

	hostkeys, err := utils.GetMetadataGuestAttribute("hostkeys/")
	if err != nil {
		t.Fatal(err)

	}
	for _, keyType := range strings.Split(hostkeys, "\n") {
		keyValue, err := utils.GetMetadataGuestAttribute("hostkeys/" + keyType)
		if err != nil {
			t.Fatal(err)
		}
		valueFromDisk, found := diskEntries[keyType]
		if !found {
			t.Fatalf("failed finding key %s from disk", keyType)
		}
		if valueFromDisk != strings.TrimSpace(keyValue) {
			t.Fatalf("host keys %s %s in guest attributes match those on disk %s", keyType, keyValue, valueFromDisk)
		}
	}
}

func TestHostKeysAreUnique(t *testing.T) {
	t.SkipNow()
}
