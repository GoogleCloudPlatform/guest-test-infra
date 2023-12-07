//go:build cit
// +build cit

package packagevalidation

import (
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// findUnique returns a slice of elements that are in arrayA but not in arrayB.
func findUniq(arrayA, arrayB []string) []string {
	unique := []string{}
	for _, itemA := range arrayA {
		if !slices.Contains(arrayB, itemA) {
			unique = append(unique, itemA)
		}
	}

	return unique
}

// removeFromArray removes a specific string from an array of strings.
func removeFromArray(arr []string, strToRemove string) []string {
	var newArr []string
	for _, item := range arr {
		if item != strToRemove {
			newArr = append(newArr, item)
		}
	}
	return newArr
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
	// Redhat like is default
	requiredPkgs := []string{"google-guest-agent", "google-osconfig-agent"}
	requiredPkgs = append(requiredPkgs, "google-compute-engine", "gce-disk-expand", "google-cloud-cli")
	requiredPkgs = append(requiredPkgs, "google-compute-engine-oslogin")

	if strings.Contains(image, "sles") || strings.Contains(image, "suse") {
		requiredPkgs = removeFromArray(requiredPkgs, "google-cloud-cli")
		requiredPkgs = removeFromArray(requiredPkgs, "google-compute-engine")
		requiredPkgs = removeFromArray(requiredPkgs, "google-compute-engine-oslogin")
		requiredPkgs = removeFromArray(requiredPkgs, "gce-disk-expand")
		requiredPkgs = append(requiredPkgs, "google-guest-configs") // SLES name for 'google-compute-engine' package.
		requiredPkgs = append(requiredPkgs, "google-guest-oslogin")
	}
	if strings.Contains(image, "centos-7") {
		requiredPkgs = append(requiredPkgs, "epel-release")
	}
	if strings.Contains(image, "debian") {
		requiredPkgs = append(requiredPkgs, "haveged", "net-tools")
		requiredPkgs = append(requiredPkgs, "google-cloud-packages-archive-keyring", "isc-dhcp-client")
		excludePkgs = append(excludePkgs, "cloud-initramfs-growroot")
	}
	if strings.Contains(image, "ubuntu") {
		requiredPkgs = removeFromArray(requiredPkgs, "gce-disk-expand")
	}

	cmd := exec.Command(listPackagesCmd[0], listPackagesCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to execute list packages command: %v", err)
		return
	}
	installedPkgs := strings.Split(string(out), "\n")

	missingPkgs := findUniq(requiredPkgs, installedPkgs)
	for _, pkg := range missingPkgs {
		// Accept google-cloud-sdk as a replacement for google-cloud-cli during migration
		if pkg == "google-cloud-cli" && slices.Contains(installedPkgs, "google-cloud-sdk") {
			t.Logf("found image with google-cloud-sdk, migrate to google-cloud-cli")
			continue
		}
		t.Errorf("Required package not installed: %s\n", pkg)
	}

	shouldnotPkgs := findUniq(excludePkgs, installedPkgs) // find whats installed
	shouldnotPkgs = findUniq(excludePkgs, shouldnotPkgs)  // it shouldn't be
	for _, pkg := range shouldnotPkgs {
		t.Errorf("Package installed but should not be: %s\n", pkg)
	}

}
