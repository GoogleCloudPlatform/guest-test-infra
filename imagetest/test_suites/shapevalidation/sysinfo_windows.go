// go:build cit && windows
//go:build cit && windows
// +build cit,windows

package shapevalidation

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type MemoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

var k32 *syscall.DLL
var numaMaskWith64Procs = false

func reliableNuma() bool {
	// If the build numer of windows is below 20348, the number of NUMA nodes is
	// not reliable for systems with 64 processors in a numa node because we can't
	// distinguish between real and fake numa nodes. This function must not be
	// called until after checking numNumaNodes. Returning false opts out of numa
	// node validation, so we return true when this is indeterminate.
	// See https://learn.microsoft.com/en-us/windows/win32/procthread/numa-support#behavior-starting-with-windows-10-build-20348
	cmd := exec.Command("powershell.exe", "systeminfo")
	out, err := cmd.Output()
	if err != nil {
		return true
	}
	BuildRe := regexp.MustCompile("Build [0-9]+")
	m := BuildRe.Find(out)
	i, err := strconv.ParseUint(strings.TrimPrefix(string(m), "Build "), 10, 64)
	if err != nil || i > 20347 {
		return true
	}
	return !numaMaskWith64Procs
}

func k32Proc(proc string) (*syscall.Proc, error) {
	if k32 == nil {
		k, err := syscall.LoadDLL("kernel32.dll")
		if err != nil {
			return nil, err
		}
		k32 = k
	}
	return k32.FindProc(proc)
}

func memTotal() (uint64, error) {
	var msx MemoryStatusEx
	msx.dwLength = uint32(unsafe.Sizeof(msx))

	globalMemoryStatusEx, err := k32Proc("GlobalMemoryStatusEx")
	if err != nil {
		return 0, err
	}

	r, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&msx)))
	if r == 0 {
		return 0, err
	}
	return (msx.ullTotalPhys / 1_000_000_000), nil
}

func numCpus() (int, error) {
	getActiveProcessorCount, err := k32Proc("GetActiveProcessorCount")
	if err != nil {
		return 0, err
	}
	// Requesting active processors in group 0xFFFF gives all active processors.
	r, _, err := getActiveProcessorCount.Call(uintptr(65535))
	if r != 0 {
		// The return code is zero on error, otherwise the processor count. Error is
		// always non-nil because of win32 API semantics.
		err = nil
	}
	return int(r), err
}

func numNumaNodes() (uint8, error) {
	// There is no function to list the number of nodes on a system,
	// and they are not guaranteed to be a sequential list, so we are
	// counting all valid node numbers up to the highest possible node
	// number.
	highestNode, err := k32Proc("GetNumaHighestNodeNumber")
	if err != nil {
		return 0, err
	}
	procMask, err := k32Proc("GetNumaNodeProcessorMaskEx")
	if err != nil {
		return 0, err
	}
	var hn uint32
	r, _, err := highestNode.Call(uintptr(unsafe.Pointer(&hn)))
	if r == 0 {
		return 0, err
	}
	var count, i uint8
	var bits uint64
	for i = 0; i <= uint8(hn); i++ {
		r, _, _ := procMask.Call(uintptr(i), uintptr(unsafe.Pointer(&bits)))
		if r != 0 {
			count++
			if bits == 0xffffffffffffffff {
				//64 processors in this numa node. See func reliableNuma.
				numaMaskWith64Procs = true
			}
		}
	}
	return count, nil
}
