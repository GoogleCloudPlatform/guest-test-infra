package shutdown_scripts

import (
	"fmt"
	"os"
	"testing"
)

// TestScriptTime tests that shutdown scripts can continue running for around
// two minutes. Because the test involves a reboot, the test context must be
// permissive of reboots (i.e. don't cancel or clean up on reboot), and this
// test will run both times so it must recognize when the second boot has
// occurred. It is a good example of a single-VM, complicated test.
//
func TestScriptTimeOrig(t *testing.T) {
	// old logic:
	// instance was started in setupClass
	// stop instance
	// start instance
	// get some output file on the instance
	// read the file and see if the script was able to write that long
	//
	return
}

func parseFile() error {
	fmt.Println("TestSuccess: The file exists!")
	return nil
}

var filename = "/root/the_log"

func TestScriptTime(t *testing.T) {
	/*
		creds, err := google.FindDefaultCredentials(context.Background())
		if err != nil {
			t.Fatalf("%v", err)
		}
		t.Logf("got: %v", creds.TokenSource)
	*/
	_, err := os.Stat(filename)
	if err == nil {
		if err := parseFile(); err != nil {
			t.Errorf("It failed..")
		}
		return
	}
	if os.IsNotExist(err) {
		fmt.Printf("TestInterimSuccess: No file to parse, probably first boot")
		// If I exit, won't that cause an issue... bc my results will get uploaded?
		return

	}

}
