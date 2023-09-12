// go:build cit && linux
//go:build cit && linux
// +build cit,linux

package shapevalidation

import (
	"os"
	"regexp"
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
	var cpulist = make(map[string]any)
	possibleCpu, err := os.ReadDir("/sys/devices/system/cpu")
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile("cpu[0-9]+")
	for _, c := range possibleCpu {
		if re.MatchString(c.Name()) {
			cpulist[c.Name()] = struct{}{}
		}
	}
	return (len(cpulist)), nil
}

func numNumaNodes() (uint8, error) {
	var nodelist = make(map[string]any)
	possibleNodes, err := os.ReadDir("/sys/devices/system/node")
	if err != nil {
		return 0, err
	}
	re := regexp.MustCompile("node[0-9]+")
	for _, n := range possibleNodes {
		if re.MatchString(n.Name()) {
			nodelist[n.Name()] = struct{}{}
		}
	}
	return uint8(len(nodelist)), nil
}
