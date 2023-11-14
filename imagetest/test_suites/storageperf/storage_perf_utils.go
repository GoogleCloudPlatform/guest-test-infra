package storageperf

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// PerformanceTargets is a structure which stores the expected iops for each operation. This is used as a value in a map from machine type to performance targets
type PerformanceTargets struct {
	randReadIOPS  float64
	randWriteIOPS float64
	seqReadBW     float64
	seqWriteBW    float64
}

const (
	vmName = "vm"
	// iopsErrorMargin allows for a small difference between iops found in the test and the iops value listed in public documentation.
	iopsErrorMargin = 0.85
	mountdiskSizeGB = 3500
	bootdiskSizeGB  = 50
	bytesInMB       = 1048576
	mountDiskName   = "hyperdisk"
	fioCmdNameLinux = "fio"
	// The fixed gcs location where fio.exe is stored.
	fioWindowsGCS = "gs://gce-image-build-resources/windows/fio.exe"
	// The local path on the test VM where fio is stored.
	fioWindowsLocalPath = "C:\\fio.exe"
	// constants for the mode of running the test
	randomMode     = "random"
	sequentialMode = "sequential"
	// Guest Attribute constants for storing the expected iops
	randReadAttribute  = "randRead"
	randWriteAttribute = "randWrite"
	seqReadAttribute   = "seqRead"
	seqWriteAttribute  = "seqWrite"
)

// map the machine type to performance targets
var hyperdiskIOPSMap = map[string]PerformanceTargets{
	"c3-standard-88": {
		randReadIOPS:  350000.0,
		randWriteIOPS: 350000.0,
		seqReadBW:     5000.0,
		seqWriteBW:    5000.0,
	},
	"c3d-standard-180": {
		randReadIOPS:  350000.0,
		randWriteIOPS: 350000.0,
		seqReadBW:     5000.0,
		seqWriteBW:    5000.0,
	},
	"n2-standard-80": {
		randReadIOPS:  160000.0,
		randWriteIOPS: 160000.0,
		seqReadBW:     5000.0,
		seqWriteBW:    5000.0,
	},
}

var pdbalanceIOPSMap = map[string]PerformanceTargets{
	"c3-standard-88": {
		randReadIOPS:  80000.0,
		randWriteIOPS: 80000.0,
		seqReadBW:     1200.0,
		seqWriteBW:    1200.0,
	},
	"c3d-standard-180": {
		randReadIOPS:  80000.0,
		randWriteIOPS: 80000.0,
		seqReadBW:     2200.0,
		seqWriteBW:    2200.0,
	},
	"n2d-standard-64": {
		randReadIOPS:  80000.0,
		randWriteIOPS: 80000.0,
		seqReadBW:     1200.0,
		seqWriteBW:    1200.0,
	},
	// this machine type should use Intel Skylake
	"n1-standard-64": {
		randReadIOPS:  80000.0,
		randWriteIOPS: 80000.0,
		seqReadBW:     1200.0,
		seqWriteBW:    1200.0,
	},
	"h3-standard-88": {
		randReadIOPS:  15000.0,
		randWriteIOPS: 15000.0,
		seqReadBW:     240.0,
		seqWriteBW:    240.0,
	},
	"t2a-standard-48": {
		randReadIOPS:  80000.0,
		randWriteIOPS: 80000.0,
		seqReadBW:     1800.0,
		seqWriteBW:    1800.0,
	},
}

// FIOOutput defines the output from the fio command
type FIOOutput struct {
	Jobs []FIOJob               `json:"jobs,omitempty"`
	X    map[string]interface{} `json:"-"`
}

// FIOJob defines one of the jobs listed in the FIO output.
type FIOJob struct {
	ReadResult  FIOStatistics          `json:"read,omitempty"`
	WriteResult FIOStatistics          `json:"write,omitempty"`
	X           map[string]interface{} `json:"-"`
}

// FIOStatistics give information about FIO performance.
type FIOStatistics struct {
	// BandwidthBytes should be able to convert to an int64
	BandwidthBytes json.Number `json:"bw_bytes,omitempty"`
	// IOPS should be able to convert to a float64
	IOPS json.Number            `json:iops,omitempty"`
	X    map[string]interface{} `json:"-"`
}

// installFioWindows copies the fio.exe file onto the VM instance.
func installFioWindows() error {
	if procStatus, err := utils.RunPowershellCmd("gsutil cp " + fioWindowsGCS + " " + fioWindowsLocalPath); err != nil {
		return fmt.Errorf("gsutil failed with error: %v %s %s", err, procStatus.Stdout, procStatus.Stderr)
	}
	return nil
}

// installFioLinux tries to install fio on linux with any of multiple package managers, and returns an error if all the package managers were not found or failed.
func installFioLinux() error {
	var installFioCmd *exec.Cmd
	if utils.CheckLinuxCmdExists("apt") {
		// only run update if using apt
		if _, err := exec.Command("apt", "-y", "update").CombinedOutput(); err != nil {
			return fmt.Errorf("apt update failed with error: %v", err)
		}
		installFioCmd = exec.Command("apt", "install", "-y", fioCmdNameLinux)
	} else if utils.CheckLinuxCmdExists("dnf") {
		installFioCmd = exec.Command("dnf", "-y", "install", fioCmdNameLinux)
	} else if utils.CheckLinuxCmdExists("yum") {
		installFioCmd = exec.Command("yum", "-y", "install", fioCmdNameLinux)
	} else if utils.CheckLinuxCmdExists("zypper") {
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", fioCmdNameLinux)
	} else {
		return fmt.Errorf("no package managers to install fio found")
	}

	if _, err := installFioCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install fio command failed with errors: %v", err)
	}
	return nil
}
