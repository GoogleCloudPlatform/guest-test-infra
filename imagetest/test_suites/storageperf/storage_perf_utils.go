package storageperf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
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
	bootdiskSizeGB  = 50
	bytesInMB       = 1048576
	mountDiskName   = "hyperdisk"
	fioCmdNameLinux = "fio"
	// constant from the fio docs to convert bandwidth to bw_bytes:
	// https://fio.readthedocs.io/en/latest/fio_doc.html#json-output
	fioBWToBytes = 1024
	// The fixed gcs location where fio.exe is stored.
	fioWindowsGCS = "gs://gce-image-build-resources/windows/fio.exe"
	// The local path on the test VM where fio is stored.
	fioWindowsLocalPath = "C:\\fio.exe"
	// constants for the mode of running the test
	randRead  = "randread"
	randWrite = "randwrite"
	seqRead   = "read"
	seqWrite  = "write"
	// Guest Attribute constants for storing the expected iops and disk type
	diskTypeAttribute  = "diskType"
	randReadAttribute  = "randRead"
	randWriteAttribute = "randWrite"
	seqReadAttribute   = "seqRead"
	seqWriteAttribute  = "seqWrite"
	// disk size varies due to performance limits per GB being different for disk types
	diskSizeGBAttribute = "diskSizeGB"
	// this excludes the filename=$TEST_DIR and filesize=$SIZE_IN_GB fields, which should be manually added to the string
	fillDiskCommonOptions   = "--name=fill_disk --direct=1 --verify=0 --randrepeat=0 --bs=128K --iodepth=64 --rw=randwrite --iodepth_batch_submit=64  --iodepth_batch_complete_max=64"
	commonFIORandOptions    = "--name=write_iops_test --filesize=500G --numjobs=1 --time_based --runtime=1m --ramp_time=2s --direct=1 --verify=0 --bs=4K --iodepth=256 --randrepeat=0 --iodepth_batch_submit=256  --iodepth_batch_complete_max=256 --output-format=json"
	commonFIOSeqOptions     = "--name=write_bandwidth_test --filesize=500G --time_based --ramp_time=2s --runtime=1m --direct=1 --verify=0 --randrepeat=0 --numjobs=1 --offset_increment=500G --bs=1M --iodepth=64 --iodepth_batch_submit=64 --iodepth_batch_complete_max=64 --output-format=json"
	hyperdiskFIORandOptions = "--numjobs=8 --size=500G --time_based --runtime=5m --ramp_time=10s --direct=1 --verify=0 --bs=4K --iodepth=256 --iodepth_batch_submit=256 --iodepth_batch_complete_max=256 --group_reporting --output-format=json"
	hyperdiskFIOSeqOptions  = "--numjobs=8 --size=500G --time_based --runtime=5m --ramp_time=10s --direct=1 --verify=0 --bs=1M --iodepth=64 --iodepth_batch_submit=64 --iodepth_batch_complete_max=64 --offset_increment=20G --group_reporting --output-format=json"
	lssdFIORandOptions      = "--name=write_iops_test --numjobs=8 --size=500G --time_based --runtime=5m --randrepeat=0 --invalidate=1 --ramp_time=10s --direct=1 --verify=0 --verify_fatal=0 --bs=4K --iodepth=256 --iodepth_batch_submit=256 --iodepth_batch_complete_max=256 --group_reporting --output-format=json"
	lssdFIOSeqOptions       = "--name=write_bandwidth_test --numjobs=8 --size=500G --time_based --runtime=5m --randrepeat=0 --invalidate=1 --ramp_time=10s --direct=1 --verify=0 --verify_fatal=0 --bs=1M --iodepth=64 --iodepth_batch_submit=64 --iodepth_batch_complete_max=64 --offset_increment=20G --group_reporting --output-format=json"
)

// map the machine type to performance targets
var lssdIOPSMap = map[string]PerformanceTargets{
	"c3-standard-88-lssd": {
		randReadIOPS:  1600000.0,
		randWriteIOPS: 800000.0,
		seqReadBW:     6240.0,
		seqWriteBW:    3120.0,
	},
	"c3d-standard-180": {
		randReadIOPS:  1600000.0,
		randWriteIOPS: 800000.0,
		seqReadBW:     6240.0,
		seqWriteBW:    3120.0,
	},
}

var hyperdiskExtremeIOPSMap = map[string]PerformanceTargets{
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

// The mount disk size should be large enough that size*iopsPerGB is equal to the iops performance target
// https://cloud.google.com/compute/docs/disks/performance#iops_limits_for_zonal
// https://cloud.google.com/compute/docs/disks/hyperdisks#iops_for
var iopsPerGBMap = map[string]int{
	imagetest.HyperdiskExtreme: 1000,
	imagetest.PdBalanced:       6,
	"lssd":                     0, // This value shouldn't be needed, but LSSD performance should be unaffected by core count.
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

// installPkgLinux tries to install a package on linux with any of multiple package managers, and returns an error if all the package managers were not found or failed.
func installPkgLinux(pkg string) error {
	usingZypper := false
	var installFioCmd *exec.Cmd
	if utils.CheckLinuxCmdExists("apt") {
		// only run update if using apt
		if _, err := exec.Command("apt", "-y", "update").CombinedOutput(); err != nil {
			return fmt.Errorf("apt update failed with error: %v", err)
		}
		installFioCmd = exec.Command("apt", "install", "-y", pkg)
	} else if utils.CheckLinuxCmdExists("dnf") {
		installFioCmd = exec.Command("dnf", "-y", "install", pkg)
	} else if utils.CheckLinuxCmdExists("yum") {
		installFioCmd = exec.Command("yum", "-y", "install", pkg)
	} else if utils.CheckLinuxCmdExists("zypper") {
		usingZypper = true
		installFioCmd = exec.Command("zypper", "--non-interactive", "install", pkg)
	} else {
		return fmt.Errorf("no package managers to install %s found", pkg)
	}

	// print more detailed error message than "exit code 1"
	var out bytes.Buffer
	var stderr bytes.Buffer
	installFioCmd.Stdout = &out
	installFioCmd.Stderr = &stderr
	if err := installFioCmd.Start(); err != nil {
		return fmt.Errorf("install %s command failed to start: err %v, %s, %s", pkg, err, out.String(), stderr.String())
	}

	if err := installFioCmd.Wait(); err != nil {
		stdoutStr := out.String()
		stderrStr := stderr.String()
		// Transient backend issues with zypper can cause exit errors 7, 104, 106, etc. Return a more detailed error message in these cases.
		if usingZypper {
			return checkZypperTransientError(err, stdoutStr, stderrStr)
		}
		return fmt.Errorf("install %s command failed with errors: %v, %s, %s", pkg, err, stdoutStr, stderrStr)
	}
	return nil
}

// Assumes the larger disk is the disk which performance is being tested on, and gets the symlink to the disk
func getLinuxSymlink(mountdiskSizeGBString string) (string, error) {
	symlinkRealPath := ""
	mountdiskSizeGB, err := strconv.Atoi(mountdiskSizeGBString)
	if err != nil {
		return "", fmt.Errorf("disk gb attribute size was not an int: %s", mountdiskSizeGBString)
	}
	diskPartition, err := utils.GetMountDiskPartition(mountdiskSizeGB)
	if err == nil {
		symlinkRealPath = "/dev/" + diskPartition
	} else {
		return "", fmt.Errorf("failed to find symlink: error %v", err)
	}
	return symlinkRealPath, nil
}

// Assemble all LSSDs in RAID, and return a platform specific identifier for the array
// On linux, returns a /dev/mdX disk path
// On windows, returns a \\.\PhysicalDriveX disk path
// See https://cloud.google.com/compute/docs/disks/benchmarking-local-ssd-performance#max_partitions
func collectLSSDs(ctx context.Context) (string, error) {
	var diskPath string
	if utils.IsWindows() {
		num, err := utils.RunPowershellCmd(`(Get-Disk -FriendlyName LssdVdisk).Number`)
		if err == nil && num.Stdout != "" {
			return `\\.\PhysicalDrive` + strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(num.Stdout), "\n"), "\r"), nil
		}
		out, err := utils.RunPowershellCmd(`New-StoragePool -FriendlyName LssdPool -ResiliencySettingNameDefault Simple -StorageSubsystemFriendlyName "Windows Storage*" -PhysicalDisks $(Get-PhysicalDisk -CanPool $True | Where-Object Model -Match "nvme_card[0-9]+") | New-VirtualDisk -FriendlyName "LssdVdisk" -UseMaximumSize`)
		if err != nil {
			return "", fmt.Errorf("failed to create lssd raid array: %s %v", out.Stdout, err)
		}
		num, err = utils.RunPowershellCmd(`(Get-Disk -FriendlyName LssdVdisk).Number`)
		if err != nil {
			return "", fmt.Errorf("could not find vdisk number: %s %v", num.Stdout, err)
		}
		return `\\.\PhysicalDrive` + strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(num.Stdout), "\n"), "\r"), nil
	}
	if !utils.CheckLinuxCmdExists("mdadm") {
		err := installPkgLinux("mdadm")
		if err != nil {
			return "", err
		}
	}
	if _, err := os.Stat("/dev/md0"); os.IsNotExist(err) {
		var mdadmargs []string
		disks, err := os.ReadDir("/dev/disk/by-id/")
		if err != nil {
			return "", err
		}
		for _, disk := range disks {
			if strings.HasPrefix(disk.Name(), "google-local-nvme-ssd") {
				mdadmargs = append(mdadmargs, "/dev/disk/by-id/"+disk.Name())
			}
		}
		mdadmargs = append([]string{"--create", "/dev/md0", "--level=0", fmt.Sprintf("--raid-devices=%d", len(mdadmargs))}, mdadmargs...)
		cmd := exec.CommandContext(ctx, "mdadm", mdadmargs...)
		o, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to run \"mdadm %s\": %s %v", mdadmargs, o, err)
		}
		diskPath = "/dev/md0"
	} else if err == nil {
		diskPath = "/dev/md0"
	} else {
		return "", fmt.Errorf("could not determine if raid array exists: %v", err)
	}
	return diskPath, nil
}

// diskClass should be one of "pd", "hyperdisk", "lssd"
func getFIOOptions(mode, diskClass string) string {
	switch diskClass {
	case "hyperdisk":
		if mode == randRead || mode == randWrite {
			return hyperdiskFIORandOptions + " --rw=" + mode
		}
		return hyperdiskFIOSeqOptions + " --rw=" + mode
	case "lssd":
		if mode == randRead || mode == randWrite {
			return lssdFIORandOptions + " --rw=" + mode
		}
		return lssdFIOSeqOptions + " --rw=" + mode
	case "pd":
		fallthrough
	default:
		if mode == randRead || mode == randWrite {
			return commonFIORandOptions + " --rw=" + mode
		}
		return commonFIOSeqOptions + " --rw=" + mode
	}
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

// use the guest attribute to check what kind of disk is being tested. If the guest attribute was not set, assume by default that PD is used.
func getDiskClass(ctx context.Context) string {
	diskType, err := utils.GetMetadata(ctx, "instance", "attributes", diskTypeAttribute)
	if err != nil {
		return "pd"
	} else if diskType == "lssd" {
		return "lssd"
	} else if diskType == imagetest.HyperdiskExtreme || diskType == imagetest.HyperdiskThroughput || diskType == imagetest.HyperdiskBalanced {
		return "hyperdisk"
	}
	return "pd"
}

// function to get num numa nodes
// TODO: implement this for windows hyperdisk
func getNumNumaNodes() (int, error) {
	if runtime.GOOS == "windows" {
		return 0, fmt.Errorf("getNumaNodes not yet implemented on windows")
	}
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
// TODO: implement this for windows hyperdisk
func getCPUNvmeMapping(symlinkRealPath string) (string, string, error) {
	if runtime.GOOS == "windows" {
		return "", "", fmt.Errorf("get cpu to nvme mapping not yet implemented on windows")
	}
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
	queue1Cpus := strings.TrimSpace(cpuListOutLines[0])
	queue2Cpus := strings.TrimSpace(cpuListOutLines[1])
	return queue1Cpus, queue2Cpus, nil
}

// fill the disk before testing to reach the maximum read iops and bandwidth
// TODO: implement this for windows by passing in the \\\\.\\PhysicalDrive1 parameter
func fillDisk(symlinkRealPath string, t *testing.T) error {
	if runtime.GOOS == "windows" {
		t.Logf("fill disk preliminary step not yet implemented for windows: performance may be lower than the target values")
	} else {
		// hard coding the filesize to 500G to save time on the fill disk step, as it
		// apppears to give sufficient performance
		fillDiskCmdOptions := fillDiskCommonOptions + " --ioengine=libaio --filesize=500G --filename=" + symlinkRealPath
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
		queue1Cpus, queue2Cpus, err := getCPUNvmeMapping(symlinkRealPath)
		if err != nil {
			return "", fmt.Errorf("could not get cpu to nvme queue mapping: err %v", err)
		}
		readOptionsSuffix += " --name=read_iops --cpus_allowed=" + queue1Cpus + " --name=read_iops_2 --cpus_allowed=" + queue2Cpus
	} else {
		readOptionsSuffix += " --name=read_iops --numa_cpu_nodes=0 --name=read_iops_2 --numa_cpu_nodes=1"
	}
	return readOptionsSuffix, nil
}

func installFioAndFillDisk(symlinkRealPath, diskClass string, t *testing.T) error {
	if err := installPkgLinux(fioCmdNameLinux); err != nil {
		return fmt.Errorf("fio installation on linux failed: err %v", err)
	}
	if err := fillDisk(symlinkRealPath, t); err != nil {
		return fmt.Errorf("fill disk preliminary step failed: err %v", err)
	}
	return nil
}

func runFIOLinux(t *testing.T, mode string) ([]byte, error) {
	ctx := utils.Context(t)
	diskClass := getDiskClass(ctx)
	options := getFIOOptions(mode, diskClass)

	var diskPath string
	if diskClass == "lssd" {
		var err error
		diskPath, err = collectLSSDs(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		mountdiskSizeGBString, err := utils.GetMetadata(utils.Context(t), "instance", "attributes", diskSizeGBAttribute)
		if err != nil {
			return []byte{}, fmt.Errorf("couldn't get image from metadata")
		}
		diskPath, err = getLinuxSymlink(mountdiskSizeGBString)
		if err != nil {
			return []byte{}, err
		}
	}
	// ubuntu 16.04 has a different option name due to an old fio version
	image, err := utils.GetMetadata(ctx, "instance", "image")
	if err != nil {
		return []byte{}, fmt.Errorf("couldn't get image from metadata")
	}
	if strings.Contains(image, "ubuntu-pro-1604") {
		options = strings.Replace(options, "iodepth_batch_complete_max", "iodepth_batch_complete", 1)
	}
	if strings.Contains(image, "ubuntu") && (strings.Contains(image, "1804") || strings.Contains(image, "1604")) {
		err := installPkgLinux("libnuma-dev")
		if err != nil {
			return nil, err
		}
	}

	if !utils.CheckLinuxCmdExists(fioCmdNameLinux) {
		if err = installFioAndFillDisk(diskPath, diskClass, t); err != nil {
			return []byte{}, err
		}
	}
	options += " --filename=" + diskPath + " --ioengine=libaio"
	// use the recommended options from the hyperdisk docs at https://cloud.google.com/compute/docs/disks/benchmark-hyperdisk-performance
	// the options --name and --numa_cpu_node must be at the very end of the command to run the jobs correctly on hyperdisk and avoid confusing fio
	if diskClass == "hyperdisk" {
		hyperdiskAdditionalOptions, err := getHyperdiskAdditionalOptions(diskPath)
		if err != nil {
			t.Fatalf("failed to get hyperdisk additional options: error %v", err)
		}
		options += hyperdiskAdditionalOptions
	}
	randCmd := exec.Command(fioCmdNameLinux, strings.Fields(options)...)
	IOPSJson, err := randCmd.CombinedOutput()
	if err != nil {
		return []byte{}, fmt.Errorf("fio command failed with error: %v %v", string(IOPSJson), err)
	}
	return IOPSJson, nil
}

func runFIOWindows(t *testing.T, mode string) ([]byte, error) {
	IOPSFile := "C:\\fio-iops.txt"
	ctx := utils.Context(t)
	// TODO: hyperdisk testing is not yet implemented for windows
	diskClass := getDiskClass(ctx)
	if diskClass == "hyperdisk" {
		diskClass = "pd"
	}
	fiopOptions := getFIOOptions(mode, diskClass)
	diskPath := `\\.\PhysicalDrive1`
	if diskClass == "lssd" {
		var err error
		diskPath, err = collectLSSDs(ctx)
		if err != nil {
			return nil, err
		}
	}
	fioOptionsWindows := " -ArgumentList \"" + fiopOptions + " --output=" + IOPSFile + " --filename=" + diskPath + " --ioengine=windowsaio" + " --thread\"" + " -wait"
	// fioWindowsLocalPath is defined within storage_perf_utils.go
	if procStatus, err := utils.RunPowershellCmd("Start-Process " + fioWindowsLocalPath + fioOptionsWindows); err != nil {
		return []byte{}, fmt.Errorf("fio.exe returned with error: %v %s %s", err, procStatus.Stdout, procStatus.Stderr)
	}

	IOPSJsonProcStatus, err := utils.RunPowershellCmd("Get-Content " + IOPSFile)
	if err != nil {
		return []byte{}, fmt.Errorf("Get-Content of fio output file returned with error: %v %s %s", err, IOPSJsonProcStatus.Stdout, IOPSJsonProcStatus.Stderr)
	}
	return []byte(IOPSJsonProcStatus.Stdout), nil
}

// get the minimum mount disk size required to reach the iops target.
// default to 3500GB if this calculation fails.
func getRequiredDiskSize(machineType, diskType string) int64 {
	if diskType == "lssd" {
		return 0
	}
	// mount disks should always be at least 3500GB, as a testing convention.
	var minimumDiskSizeGB int64 = 3500
	var iopsTargetStruct PerformanceTargets
	var iopsTargetFound bool
	if diskType == imagetest.PdBalanced {
		iopsTargetStruct, iopsTargetFound = pdbalanceIOPSMap[machineType]

	} else if diskType == imagetest.HyperdiskExtreme {
		iopsTargetStruct, iopsTargetFound = hyperdiskExtremeIOPSMap[machineType]
	}

	iopsPerGB, diskTypeFound := iopsPerGBMap[diskType]
	if iopsTargetFound && diskTypeFound {
		calculatedDiskSizeGB := int64(iopsTargetStruct.randReadIOPS / float64(iopsPerGB))
		if calculatedDiskSizeGB > minimumDiskSizeGB {
			return calculatedDiskSizeGB
		}
	}
	return minimumDiskSizeGB
}
