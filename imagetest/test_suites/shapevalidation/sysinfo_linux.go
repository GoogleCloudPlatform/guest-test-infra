// go:build cit && linux
//go:build cit && linux
// +build cit,linux

package shapevalidation

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func reliableNuma() bool {
	// Always reliable on linux, see sysinfo_windows.go
	return true
}

func memTotal() (uint64, error) {
	var info syscall.Sysinfo_t
	if err := syscall.Sysinfo(&info); err != nil {
		return 0, err
	}
	return (info.Totalram / 1_000_000_000), nil
}

func numCpus() (int, error) {
	cpus, err := os.ReadFile("/sys/devices/system/cpu/online")
	if err != nil {
		return 0, err
	}
	return countKernelList(string(cpus))
}

func numNumaNodes() (uint8, error) {
	nodes, err := os.ReadFile("/sys/devices/system/node/online")
	if err != nil {
		return 0, err
	}
	c, err := countKernelList(string(nodes))
	return uint8(c), err
}

// Parse a list of things such as nodes, cpus, etc from the kernel in the format "0-4,6"
// and return the count of items in the list.
func countKernelList(list string) (int, error) {
	var count int
	for _, item := range strings.Split(strings.TrimSpace(list), ",") {
		pair := strings.Split(item, "-")
		if len(pair) == 1 {
			count++
			continue
		}
		if len(pair) == 2 {
			i0, err := strconv.Atoi(pair[0])
			if err != nil {
				return 0, err
			}
			i1, err := strconv.Atoi(pair[1])
			if err != nil {
				return 0, err
			}
			count = count + (i1 - i0) + 1
			continue
		}
		return 0, fmt.Errorf("malformed list %q", list)
	}
	return count, nil
}
