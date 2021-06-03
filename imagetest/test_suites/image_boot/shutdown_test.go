package imageboot

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

const shutdownTime = 110 // about 2 minutes

// TestGuestShutdownScript test that shutdown scripts can run for around two minutes
func TestGuestShutdownScript(t *testing.T) {
	_, err := os.Stat(markerFile)
	if os.IsNotExist(err) {
		// first boot
		if _, err := os.Create(markerFile); err != nil {
			t.Fatalf("failed creating marker file: %v", err)
		}
		t.Fatal("marker file does not exist")
	}
	// second boot
	bytes, err := ioutil.ReadFile("/shutdown.txt")
	if err != nil {
		t.Fatalf("error reading file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(bytes), ` \t\r\n\0`), "\n")
	if len(lines) < shutdownTime {
		t.Fatalf("shut down time less than %d seconds.", shutdownTime)
	}
}
