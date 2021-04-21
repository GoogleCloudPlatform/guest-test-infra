package network

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

const (
	gceMTU                 = 1460
	defaultInterface       = "eth0"
	defaultDebianInterface = "ens4"
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

func TestDefaultMTU(t *testing.T) {
	var networkInterface string

	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}

	switch {
	case strings.Contains(image, "debian-10"):
		networkInterface = defaultDebianInterface
	default:
		networkInterface = defaultInterface
	}

	isDefault, err := isDefaultGCEMTU(networkInterface)
	if err != nil {
		t.Fatal(err.Error())
	}
	if !isDefault {
		t.Fatal(err.Error())
	}
}

func isDefaultGCEMTU(interfaceName string) (bool, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return false, fmt.Errorf("can't get network interface")
	}
	for _, i := range ifs {
		if i.Name == interfaceName {
			if i.MTU != gceMTU {
				return false, fmt.Errorf("expected MTU %d on interface %s, got MTU %s", gceMTU, i.Name, i.MTU)
			}
			return true, nil
		}
	}
	return false, fmt.Errorf("can't find network interface %s", interfaceName)
}
