package disk

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var testData = []byte("test-data")

const markerFile = "/boot-marker"

func TestBasicRootFromPD(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// boot from first vm
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		diskType, err := utils.GetMetadata("disks/0/type")
		if err != nil {
			t.Fatalf("couldn't get disk type from metadata")
		}
		if diskType != "PERSISTENT" {
			t.Fatalf("disk is not PERSISTENT type")
		}
		if err := ioutil.WriteFile("/temp.txt", testData, 0755); err != nil {
			t.Fatalf("could not write data %v", err)
		}
		if _, err := ioutil.ReadFile("/temp.txt"); err != nil {
			t.Fatalf("could not read data %v", err)
		}
	}
	// boot from second vm
	t.Log("marker file exist signal the second boot in second vm")
	read, err := ioutil.ReadFile("/temp.txt")
	if err != nil {
		t.Fatalf("could not read data %v", err)
	}
	if !bytes.Equal(read, testData) {
		t.Fatal(" the data was not the same when pd move between vm")
	}
}
