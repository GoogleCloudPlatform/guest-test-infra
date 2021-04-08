package shutdown_scripts

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"testing"
)

var (
	minimumSeconds = 110
)

func parseFile() error {
	res, err := ioutil.ReadFile(timerfile)
	if err != nil {
		return err
	}
	lines := strings.Split(string(res), "\n")
	if len(lines) < 1 {
		return fmt.Errorf("empty file")
	}
	count := 0
	for i := len(lines) - 1; i != 0; i-- {
		icount, err := strconv.Atoi(lines[i])
		if err == nil {
			// This file can easily be corrupted. Stop on the first (last) valid line.
			count = icount
			break
		}
	}
	if count < minimumSeconds {
		return fmt.Errorf("shutdown script reported %d seconds runtime, less than minimum %d seconds", count, minimumSeconds)
	}

	return nil
}

func TestScriptTime(t *testing.T) {
	_, err := os.Stat(timerfile)
	if err == nil {
		if err := parseFile(); err != nil {
			t.Errorf("failed to parse timer file: %v", err)
		}
	} else if os.IsNotExist(err) {
		t.Log("timer file missing, assuming this is first boot")
		fmt.Println("first boot done")
		return

	}
	fmt.Println("done")

}
