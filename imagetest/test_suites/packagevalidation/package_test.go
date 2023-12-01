//go:build cit
// +build cit

package packagevalidation

import (
	"bufio"
	"bytes"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// findUnique returns a slice of elements that are in arrayA but not in arrayB.
func findUniq(arrayA, arrayB []string) []string {
	unique := []string{}
	for _, itemA := range arrayA {
		if !contains(arrayB, itemA) {
			unique = append(unique, itemA)
		}
	}
	return unique
}

// contains checks if a slice contains a particular string.
func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func TestStandardPrograms(t *testing.T) {
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")
	if err != nil {
		t.Fatalf("couldn't get image from metadata")
	}
	if strings.Contains(image, "sles") || strings.Contains(image, "suse") {
		// SLES/SUSE does not have the Google Cloud SDK installed.
		t.Skip("Cloud SDK Not supported on SLES/SUSE")
	}

	cmd := exec.Command("gcloud", "-h")
	cmd.Start()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("gcloud not installed properly")
	}
	cmd = exec.Command("gsutil", "help")
	cmd.Start()
	err = cmd.Wait()
	if err != nil {
		t.Fatalf("gsutil not installed properly")
	}
}

func TestGuestPackages(t *testing.T) {
	utils.LinuxOnly(t)
	image, err := utils.GetMetadata(utils.Context(t), "instance", "image")

	if err != nil {
		t.Fatalf("couldn't determine image from metadata")
	}

	// What command to list all packages
	listPackagesCmd := []string{"rpm", "-qa", "--queryformat", "%{NAME}\n"}
	if strings.Contains(image, "debian") || strings.Contains(image, "ubuntu") {
		listPackagesCmd = []string{"dpkg-query", "-W", "--showformat", "${Package}\n"}
	}

	// What packages to insure are not installed
	excludePkgs := []string{}

	// What packages to check for and excude for diffrent images
	requiredPkgs := []string{"google-guest-agent", "google-osconfig-agent"}
	if strings.Contains(image, "rhel") {
		requiredPkgs = append(requiredPkgs, "google-compute-engine", "gce-disk-expand", "google-cloud-sdk")
	}
	if strings.Contains(image, "sles") || strings.Contains(image, "suse") {
		requiredPkgs = append(requiredPkgs, "google-guest-configs") // SLES name for 'google-compute-engine' package.
		requiredPkgs = append(requiredPkgs, "google-guest-oslogin")
	}
	if strings.Contains(image, "centos-7") {
		requiredPkgs = append(requiredPkgs, "epel-release")
	}
	if strings.Contains(image, "debian") {
		requiredPkgs = append(requiredPkgs, "haveged", "net-tools")
		requiredPkgs = append(requiredPkgs, "google-cloud-packages-archive-keyring", "isc-dhcp-client")
		requiredPkgs = append(requiredPkgs, "google-compute-engine", "gce-disk-expand", "python3", "google-cloud-cli")
		excludePkgs = append(excludePkgs, "cloud-initramfs-growroot")
		if runtime.GOARCH == "amd64" {
			requiredPkgs = append(requiredPkgs, "grub-efi-amd64-signed", "shim-signed")
		}
		if runtime.GOARCH == "arm64" {
			requiredPkgs = append(requiredPkgs, "grub-efi-arm64")
		}
	}

	cmd := exec.Command(listPackagesCmd[0], listPackagesCmd[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out

	err = cmd.Run()
	if err != nil {
		t.Errorf("Failed to execute list packages command: %v", err)
		return
	}

	scanner := bufio.NewScanner(&out)
	var installedPkgs []string
	for scanner.Scan() {
		installedPkgs = append(installedPkgs, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading list packages command output: %v", err)
	}

	missingPkgs := findUniq(requiredPkgs, installedPkgs)
	for _, pkg := range missingPkgs {
		t.Errorf("Required package not installed: %s\n", pkg)
	}

	shouldnotPkgs := findUniq(excludePkgs, installedPkgs) // find whats installed
	shouldnotPkgs = findUniq(excludePkgs, shouldnotPkgs)  // it shouldn't be
	for _, pkg := range shouldnotPkgs {
		t.Errorf("Package installed but should not be: %s\n", pkg)
	}

}
