package disk

import (
	"io/ioutil"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func TestBasicRootFromPD(t *testing.T) {
	diskType, err := utils.GetMetadata("disks/0/type")
	if err != nil {
		t.Fatalf("couldn't get disk type from metadata")
	}

	if diskType != "PERSISTENT" {
		t.Fatal("disk is not PERSISTENT type")
	}
	if err := ioutil.WriteFile("/temp.txt", []byte("test-data"), 0755); err != nil {
		t.Fatal("could not write data %v", err)
	}
	if _, err := ioutil.ReadFile("/temp.txt"); err != nil {
		t.Fatal("could not read data %v", err)
	}
	// TODO: Test using first vm PD boot from second vm doesn't change data
}
