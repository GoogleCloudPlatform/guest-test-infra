package shutdown_scripts

import (
	"errors"
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
		return errors.New("empty file")
	}
	count, err := strconv.Atoi(lines[len(lines)-1])
	if err != nil {
		return err
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
		return
	}
	if os.IsNotExist(err) {
		t.Log("timer file missing, assuming this is first boot")
		return

	}

}
