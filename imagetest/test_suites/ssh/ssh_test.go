package ssh

import (
	"flag"
	"os"
	"testing"
)

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

func TestVm1(t *testing.T) {
	t.Log("Success")
}

func TestVm2(t *testing.T) {
	t.Log("Success")
}
