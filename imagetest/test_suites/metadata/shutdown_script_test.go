package metadata

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const markerFile = "/boot-marker"

// TestShutdownScript test the standard metadata script.
func TestShutdownScript(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	contents, err := ioutil.ReadFile(outputPath)
	if string(contents) != shutdownContent {
		t.Fatalf("shutdown script does not run succesfully")
	}
}

// TestRandomShutdownScriptNotCrashVM test that a script with random content
// doesn't crash the vm.
func TestRandomShutdownScriptNotCrashVM(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	if _, err := utils.GetMetadataAttribute("shutdown-script"); err != nil {
		t.Fatalf("couldn't get shutdown-script from metadata")
	}
}

// TestShutdownUrlScript test that URL scripts work correctly.
func TestShutdownUrlScript(t *testing.T) {
	// TODO
}
