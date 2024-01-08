package storageperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

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
	// fio should use the full disk size as the filesize when benchmarking
	mountdiskSizeGBString = "3500"
	mountdiskSizeGB       = 3500
	bootdiskSizeGB        = 50
	bytesInMB             = 1048576
	mountDiskName         = "hyperdisk"
	fioCmdNameLinux       = "fio"
	// constant from the fio docs to convert bandwidth to bw_bytes:
	// https://fio.readthedocs.io/en/latest/fio_doc.html#json-output
	fioBWToBytes = 1024
	// The fixed gcs location where fio.exe is stored.
	fioWindowsGCS = "gs://gce-image-build-resources/windows/fio.exe"
	// The local path on the test VM where fio is stored.
	fioWindowsLocalPath = "C:\\fio.exe"
	// constants for the mode of running the test
	randomMode     = "random"
	sequentialMode = "sequential"
	// Guest Attribute constants for storing the expected iops and disk type
	diskTypeAttribute  = "diskType"
	randReadAttribute  = "randRead"
	randWriteAttribute = "randWrite"
	seqReadAttribute   = "seqRead"
	seqWriteAttribute  = "seqWrite"
	// this excludes the filename=$TEST_DIR and filesize=$SIZE_IN_GB fields, which should be manually added to the string
	fillDiskCommonOptions = "--name=fill_disk --ioengine=libaio --direct=1 --verify=0 --randrepeat=0 --bs=128K --iodepth=64 --rw=randwrite --iodepth_batch_submit=64  --iodepth_batch_complete_max=64"
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
	// Bandwidth should be able to convert to an int64
	Bandwidth json.Number `json:"bw,omitempty"`
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
	usingZypper := false
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
		usingZypper = true
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", fioCmdNameLinux)
	} else {
		return fmt.Errorf("no package managers to install fio found")
	}

	// print more detailed error message than "exit code 1"
	var out bytes.Buffer
	var stderr bytes.Buffer
	installFioCmd.Stdout = &out
	installFioCmd.Stderr = &stderr
	if err := installFioCmd.Start(); err != nil {
		return fmt.Errorf("install fio command failed to start: err %v, %s, %s", err, out.String(), stderr.String())
	}

	if err := installFioCmd.Wait(); err != nil {
		stdoutStr := out.String()
		stderrStr := stderr.String()
		// Transient backend issues with zypper can cause exit errors 7, 104, 106, etc. Return a more detailed error message in these cases.
		if usingZypper {
			return checkZypperTransientError(err, stdoutStr, stderrStr)
		}
		return fmt.Errorf("install fio command failed with errors: %v, %s, %s", err, stdoutStr, stderrStr)
	}
	return nil
}

// this function is only for convenience: if a performance test fails,
// the test vm name can print out the vm machine type for faster analysis.
func getVMName(ctx context.Context) string {
	machineName, err := utils.GetMetadata(ctx, "instance", "name")
	if err != nil {
		return "unknown"
	}
	return machineName
}

// check if a known zypper backend error is found
func checkZypperTransientError(err error, stdout, stderr string) error {
	exitErr, foundErr := err.(*exec.ExitError)
	if foundErr {
		exitCode := exitErr.ExitCode()
		errorString := "zypper repo test environment setup failed: stdout " + stdout + ", stderr " + stderr + ", "
		if exitCode == 7 {
			errorString += "zypper process already running, cannot start zypper install"
		} else if exitCode == 104 {
			errorString += "fio not found within known zypper repositories after setup"
		} else if exitCode == 106 {
			errorString += "zypper repository refresh failed on setup"
		}
		return fmt.Errorf("%s, exitCode %d", errorString, exitCode)
	}
	return err
}

// function to get num numa nodes
func getNumNumaNodes() (int, error) {
	/*numaNodesCmdArgs := "| grep -i 'numa node(s)' | awk '{print $NF}'"
	numaNodesCmd := exec.Command("lscpu", strings.Fields(numaNodesCmdArgs)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	numaNodesCmd.Stdout = &stdout
	numaNodesCmd.Stderr = &stderr
	if err := numaNodesCmd.Start(); err != nil {
		return 0, err
	}
	if err := numaNodesCmd.Wait(); err != nil {
		stdoutString := stdout.String()
		stderrString := stderr.String()
		return 0, fmt.Errorf("numa nodes command failed with stdout %s and error %s, error code %v", stdoutString, stderrString, err)
	}
	numNumaNodes, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return 0, err
	}*/
	lscpuOut, err := exec.Command("lscpu").CombinedOutput()
	if err != nil {
		return 0, err
	}
	lscpuOutString := string(lscpuOut)
	numNumaNodes := -1
	for _, line := range strings.Split(lscpuOutString, "\n") {
		lowercaseLine := strings.ToLower(line)
		if strings.Contains(lowercaseLine, "numa node") {
			// the last token in the line should be the number of numa nodes
			tokens := strings.Fields(lowercaseLine)
			numNumaNodesString := strings.TrimSpace(tokens[len(tokens)-1])
			i, err := strconv.Atoi(numNumaNodesString)
			if err == nil {
				numNumaNodes = i
				break
			}
		}
	}
	if numNumaNodes < 0 {
		return 0, fmt.Errorf("did not find any line with numNumaNodes in lscpu output: %s", lscpuOutString)
	}
	return numNumaNodes, nil
}

// function to get cpu mapping as strings if there is only one numa node
// returned format is queue_1_cpus, queue_2_cpus, error
func getCpuNvmeMapping(symlinkRealPath string) (string, string, error) {
	cpuListCmd := exec.Command("cat", "/sys/class/block/"+symlinkRealPath+"/mq/*/cpu_list")
	cpuListBytes, err := cpuListCmd.CombinedOutput()
	if err != nil {
		return "", "", err
	}
	cpuListString := string(cpuListBytes)
	cpuListOutLines := strings.Split(string(cpuListString), "\n")
	if len(cpuListOutLines) < 2 {
		return "", "", fmt.Errorf("expected at least two lines for cpu queue mapping, got string %s with %d lines", cpuListString, len(cpuListOutLines))
	}
	queue_1_cpus := strings.TrimSpace(cpuListOutLines[0])
	queue_2_cpus := strings.TrimSpace(cpuListOutLines[1])
	return queue_1_cpus, queue_2_cpus, nil
}

// fill the disk before testing to reach the maximum read iops and bandwidth
// TODO: implement this for windows by passing in the \\\\.\\PhysicalDrive1 parameter
func fillDisk(symlinkRealPath string) error {
	if runtime.GOOS == "windows" {
		fmt.Println("fill disk preliminary step not yet implemented for windows")
	} else {
		// hard coding the filesize to 500G as that conforms to the docs while giving
		// sufficiently high performance
		fillDiskCmdOptions := fillDiskCommonOptions + " --filesize=500G --filename=" + symlinkRealPath
		fillDiskCmd := exec.Command(fioCmdNameLinux, strings.Fields(fillDiskCmdOptions)...)
		if err := fillDiskCmd.Start(); err != nil {
			return err
		}
		if err := fillDiskCmd.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func getHyperdiskAdditionalOptions(symlinkRealPath string) (string, error) {
	readOptionsSuffix := ""
	numNumaNodes, err := getNumNumaNodes()
	if err != nil {
		return "", fmt.Errorf("failed to get number of numa nodes: err %v", err)
	}
	if numNumaNodes == 1 {
		queue_1_cpus, queue_2_cpus, err := getCpuNvmeMapping(symlinkRealPath)
		if err != nil {
			return "", fmt.Errorf("could not get cpu to nvme queue mapping: err %v", err)
		}
		readOptionsSuffix += " --name=read_iops --cpus_allowed=" + queue_1_cpus + " --name=read_iops_2 --cpus_allowed=" + queue_2_cpus
	} else {
		readOptionsSuffix += " --name=read_iops --numa_cpu_nodes=0 --name=read_iops_2 --numa_cpu_nodes=1"
	}
	return readOptionsSuffix, nil
}
