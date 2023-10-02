package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"golang.org/x/crypto/ssh"
)

const (
	metadataURLPrefix = "http://metadata.google.internal/computeMetadata/v1/instance/"
	bytesInGB         = 1073741824
	// GuestAttributeTestNamespace is the namespace for the guest attribute in the daisy "wait for instance" step for CIT.
	GuestAttributeTestNamespace = "citTest"
	// GuestAttributeTestKey is the key for the guest attribute in the edaisy "wait for instance" step for CIT.
	GuestAttributeTestKey = "test-complete"
)

var windowsClientImagePatterns = []string{
	"windows-7-",
	"windows-8-",
	"windows-10-",
	"windows-11-",
}

// BlockDeviceList gives full information about blockdevices, from the output of lsblk.
type BlockDeviceList struct {
	BlockDevices []BlockDevice `json:"blockdevices,omitempty"`
}

// BlockDevice defines information about a single partition or disk in the output of lsblk.
type BlockDevice struct {
	Name string `json:"name,omitempty"`
	// on some OSes, size is a string, and on some OSes, size is a number.
	// This allows both to be parsed
	Size json.Number `json:"size,omitempty"`
	Type string      `json:"type,omitempty"`
	// Other fields are not currently used.
	X map[string]interface{} `json:"-"`
}

// GetRealVMName returns the real name of a VM running in the same test.
func GetRealVMName(name string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(hostname, "-", 3)
	if len(parts) != 3 {
		return "", errors.New("hostname doesn't match scheme")
	}
	return strings.Join([]string{name, parts[1], parts[2]}, "-"), nil
}

// GetMetadataAttribute returns an attribute from metadata if present, and error if not.
func GetMetadataAttribute(attribute string) (string, error) {
	return GetMetadata("attributes/" + attribute)
}

// GetMetadataGuestAttribute returns an guest attribute from metadata if present, and error if not.
func GetMetadataGuestAttribute(attribute string) (string, error) {
	return GetMetadata("guest-attributes/" + attribute)
}

// GetMetadata returns a metadata value for the specified key if it is present, and error if not.
func GetMetadata(path string) (string, error) {
	resp, err := GetMetadataHTTPResponse(path)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http response code is %v", resp.StatusCode)
	}
	val, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// GetMetadataHTTPResponse returns http response for the specified key without checking status code.
func GetMetadataHTTPResponse(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", metadataURLPrefix, path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// PutMetadataGuestAttribute sets the guest attribute in the namespace, and returns an error if this operation fails.
func PutMetadataGuestAttribute(ctx context.Context, namespace, attribute string) error {
	path, err := url.JoinPath(metadataURLPrefix, "guest-attributes", namespace, attribute)
	if err != nil {
		return err
	}
	err = PutMetadataHTTPResponse(ctx, path)
	if err != nil {
		return err
	}
	return nil
}

// PutMetadataHTTPResponse returns http response for the specified key without checking status code.
func PutMetadataHTTPResponse(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, path, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("http response code is %v", resp.StatusCode)
	}
	return nil
}

// DownloadGCSObject downloads a GCS object.
func DownloadGCSObject(ctx context.Context, client *storage.Client, gcsPath string) ([]byte, error) {
	u, err := url.Parse(gcsPath)
	if err != nil {
		log.Fatalf("Failed to parse GCS url: %v\n", err)
	}
	object := strings.TrimPrefix(u.Path, "/")
	rc, err := client.Bucket(u.Host).Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// DownloadGCSObjectToFile downloads a GCS object, writing it to the specified file.
func DownloadGCSObjectToFile(ctx context.Context, client *storage.Client, gcsPath, file string) error {
	data, err := DownloadGCSObject(ctx, client, gcsPath)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(file, data, 0755); err != nil {
		return err
	}
	return nil
}

// ExtractBaseImageName extract the base image name from full image resource.
func ExtractBaseImageName(image string) (string, error) {
	// Example: projects/rhel-cloud/global/images/rhel-8-v20210217
	splits := strings.SplitN(image, "/", 5)
	if len(splits) < 5 {
		return "", fmt.Errorf("malformed image metadata")
	}

	splits = strings.Split(splits[4], "-")
	if len(splits) < 2 {
		return "", fmt.Errorf("malformed base image name")
	}
	imageName := strings.Join(splits[:len(splits)-1], "-")
	return imageName, nil
}

// DownloadPrivateKey download private key from daisy source.
func DownloadPrivateKey(user string) ([]byte, error) {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	sourcesPath, err := GetMetadataAttribute("daisy-sources-path")
	if err != nil {
		return nil, err
	}
	gcsPath := fmt.Sprintf("%s/%s-ssh-key", sourcesPath, user)

	privateKey, err := DownloadGCSObject(ctx, client, gcsPath)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// GetHostKeysFromDisk read ssh host public key and parse
func GetHostKeysFromDisk() (map[string]string, error) {
	totalBytes, err := GetHostKeysFileFromDisk()
	if err != nil {
		return nil, err
	}
	return ParseHostKey(totalBytes)
}

// GetHostKeysFileFromDisk read ssh host public key as bytes
func GetHostKeysFileFromDisk() ([]byte, error) {
	var totalBytes []byte
	keyFiles, err := filepath.Glob("/etc/ssh/ssh_host_*_key.pub")
	if err != nil {
		return nil, err
	}

	for _, file := range keyFiles {
		bytes, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}
		totalBytes = append(totalBytes, bytes...)
	}
	return totalBytes, nil
}

// ParseHostKey parse hostkey data from bytes.
func ParseHostKey(bytes []byte) (map[string]string, error) {
	hostkeyLines := strings.Split(strings.TrimSpace(string(bytes)), "\n")
	if len(hostkeyLines) == 0 {
		return nil, fmt.Errorf("hostkey does not exist")
	}
	var hostkeyMap = make(map[string]string)
	for _, hostkey := range hostkeyLines {
		splits := strings.Split(hostkey, " ")
		if len(splits) < 2 {
			return nil, fmt.Errorf("hostkey has wrong format %s", hostkey)
		}
		keyType := strings.Split(hostkey, " ")[0]
		keyValue := strings.Split(hostkey, " ")[1]
		hostkeyMap[keyType] = keyValue
	}
	return hostkeyMap, nil
}

// CreateClient create a ssh client to connect host.
func CreateClient(user, host string, pembytes []byte) (*ssh.Client, error) {
	// generate signer instance from plain key
	signer, err := ssh.ParsePrivateKey(pembytes)
	if err != nil {
		return nil, fmt.Errorf("parsing plain private key failed %v", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host, sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetInterfaceByMAC returns the interface with the specified MAC address.
func GetInterfaceByMAC(mac string) (net.Interface, error) {
	hwaddr, err := net.ParseMAC(mac)
	if err != nil {
		return net.Interface{}, err
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return net.Interface{}, err
	}

	for _, iface := range interfaces {
		if iface.HardwareAddr.String() == hwaddr.String() {
			return iface, nil
		}
	}
	return net.Interface{}, fmt.Errorf("no interface found with MAC %s", mac)
}

// GetInterface returns the interface corresponding to the metadata interface array at the specified index.
func GetInterface(index int) (net.Interface, error) {
	mac, err := GetMetadata(fmt.Sprintf("network-interfaces/%d/mac", index))
	if err != nil {
		return net.Interface{}, err
	}

	return GetInterfaceByMAC(mac)
}

// CheckLinuxCmdExists checks that a command exists on the linux image, and is executable.
func CheckLinuxCmdExists(cmd string) bool {
	cmdPath, err := exec.LookPath(cmd)
	// returns nil prior to go 1.19, exec.ErrDot after
	if errors.Is(err, exec.ErrDot) || err == nil {
		cmdFileInfo, err := os.Stat(cmdPath)
		cmdFileMode := cmdFileInfo.Mode()
		// check the the file has executable permissions.
		if err == nil {
			return cmdFileMode&0111 != 0
		}
	}
	return false
}

// WindowsOnly skips tests not on Windows.
func WindowsOnly(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Test only run on Windows.")
	}
}

// Is32BitWindows returns true if the image contains -x86.
func Is32BitWindows(image string) bool {
	return strings.Contains(image, "-x86")
}

// Skip32BitWindows skips tests on 32-bit client images.
func Skip32BitWindows(t *testing.T, skipMsg string) {
	image, err := GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata: %v", err)
	}

	if Is32BitWindows(image) {
		t.Skip(skipMsg)
	}
}

// IsWindows returns true if the detected runtime environment is Windows.
func IsWindows() bool {
	if runtime.GOOS == "windows" {
		return true
	}
	return false
}

// IsWindowsClient returns true if the image is a client (non-server) Windows image.
func IsWindowsClient(image string) bool {
	for _, pattern := range windowsClientImagePatterns {
		if strings.Contains(image, pattern) {
			return true
		}
	}
	return false
}

// WindowsContainersOnly skips tests not on Windows "for Containers" images.
func WindowsContainersOnly(t *testing.T) {
	WindowsOnly(t)
	image, err := GetMetadata("image")
	if err != nil {
		t.Fatalf("Couldn't get image from metadata: %v", err)
	}

	if !strings.Contains(image, "-for-containers") {
		t.Skip("Test only run on Windows for Containers images")
	}
}

// ProcessStatus holds stdout, stderr and the exit code from an external command call.
type ProcessStatus struct {
	Stdout   string
	Stderr   string
	Exitcode int
}

// RunPowershellCmd runs a powershell command and returns stdout and stderr if successful.
func RunPowershellCmd(command string) (ProcessStatus, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := ProcessStatus{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Exitcode: cmd.ProcessState.ExitCode(),
	}

	return output, err
}

// CheckPowershellSuccess returns an error if the powershell command fails.
func CheckPowershellSuccess(command string) error {
	output, err := RunPowershellCmd(command)
	if err != nil {
		return err
	}

	if output.Exitcode != 0 {
		return fmt.Errorf("Non-zero exit code: %d", output.Exitcode)
	}

	return nil
}

// CheckPowershellReturnCode returns an error if the exit code doesn't match the expected value.
func CheckPowershellReturnCode(command string, want int) error {
	output, _ := RunPowershellCmd(command)

	if output.Exitcode == want {
		return nil
	}

	return fmt.Errorf("Exit Code not as expected: want %d, got %d", want, output.Exitcode)

}

// FailOnPowershellFail fails the test if the powershell command fails.
func FailOnPowershellFail(command string, errorMsg string, t *testing.T) {
	err := CheckPowershellSuccess(command)
	if err != nil {
		t.Fatalf("%s: %v", errorMsg, err)
	}
}

// GetMountDiskPartition runs lsblk to get the partition of the mount disk on linux, assuming the
// size of the mount disk is diskExpectedSizeGb.
func GetMountDiskPartition(diskExpectedSizeGB int) (string, error) {
	var diskExpectedSizeBytes int64 = int64(diskExpectedSizeGB) * int64(bytesInGB)
	lsblkCmd := "lsblk"
	if !CheckLinuxCmdExists(lsblkCmd) {
		return "", fmt.Errorf("could not find lsblk")
	}
	diskType := "disk"
	lsblkout, err := exec.Command(lsblkCmd, "-b", "-o", "name,size,type", "--json").CombinedOutput()
	if err != nil {
		errorString := err.Error()
		// execute lsblk without json as a backup
		lsblkout, err = exec.Command(lsblkCmd, "-b", "-o", "name,size,type").CombinedOutput()
		if err != nil {
			errorString += err.Error()
			return "", fmt.Errorf("failed to execute lsblk with and without json: %s", errorString)
		}
		lsblkoutlines := strings.Split(string(lsblkout), "\n")
		for _, line := range lsblkoutlines {
			linetokens := strings.Fields(line)
			if len(linetokens) != 3 {
				continue
			}
			// we should have a slice of length 3, with fields name, size, type. Search for the line with the partition of the correct size.
			var blkname, blksize, blktype = linetokens[0], linetokens[1], linetokens[2]
			blksizeInt, err := strconv.ParseInt(blksize, 10, 64)
			if err != nil {
				continue
			}
			if blktype == diskType && blksizeInt == diskExpectedSizeBytes {
				return blkname, nil
			}
		}
		return "", fmt.Errorf("failed to find disk partition with expected size %d", diskExpectedSizeBytes)
	}

	var blockDevices BlockDeviceList
	if err := json.Unmarshal(lsblkout, &blockDevices); err != nil {
		return "", fmt.Errorf("failed to unmarshal lsblk output %s with error: %v", lsblkout, err)
	}
	for _, blockDev := range blockDevices.BlockDevices {
		// deal with the case where the unmarshalled size field can be a number with or without quotes on different operating systems.
		blockDevSizeInt, err := blockDev.Size.Int64()
		if err != nil {
			return "", fmt.Errorf("block dev size %s was not an int: error %v", blockDev.Size.String(), err)
		}
		if strings.ToLower(blockDev.Type) == diskType && blockDevSizeInt == diskExpectedSizeBytes {
			return blockDev.Name, nil
		}
	}

	return "", fmt.Errorf("disk block with size not found")
}

// GetMountDiskPartitionSymlink uses symlinks to get the partition of the mount disk
// on linux, assuming the name of the disk resource is mountDiskName.
func GetMountDiskPartitionSymlink(mountDiskName string) (string, error) {
	mountDiskSymlink := "/dev/disk/by-id/google-" + mountDiskName
	symlinkRealPath, err := filepath.EvalSymlinks(mountDiskSymlink)
	if err != nil {
		return "", fmt.Errorf("symlink could not be resolved with error: %v", err)
	}
	return symlinkRealPath, nil
}
