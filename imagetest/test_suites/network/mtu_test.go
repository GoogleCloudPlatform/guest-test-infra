package network

import (
	"fmt"
	"net"
	"testing"
)

const (
	GceMtu                        = 1460
	defaultInterface              = "eth0"
	defaultDebianInterface        = "eth4"
	DefaultInterfaceWin2012Beyond = "Ethernet"
	DefaultInterfaceWin2008       = "Local Area Connection"
)

func TestLinuxDefaultMTU(t *testing.T) {
	err := checkDefaultMTU(defaultInterface)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindows2008DefaultMTU(t *testing.T) {
	err := checkDefaultMTU(DefaultInterfaceWin2008)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWin2012BeyondDefaultMTU(t *testing.T) {
	err := checkDefaultMTU(DefaultInterfaceWin2012Beyond)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDebianDefaultMTU(t *testing.T) {
	err := checkDefaultMTU(defaultDebianInterface)
	if err != nil {
		t.Fatal(err)
	}
}

func checkDefaultMTU(defaultInterface string) error {
	ifs, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("can't get network interface")
	}
	for _, i := range ifs {
		if i.Name == defaultInterface {
			if i.MTU != GceMtu {
				return fmt.Errorf("Expected MTU %d on interface %s, got MTU %s", GceMtu, i.Name, i.MTU)
			}
			return nil
		}
	}
	return fmt.Errorf("can't find network interface %s", defaultInterface)
}
