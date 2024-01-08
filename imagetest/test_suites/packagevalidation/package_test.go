//go:build cit
// +build cit

package packagevalidation

import (
	"os/exec"
	"slices"
	"fmt"
	"regexp"
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
	listPkgs := func()([]string, error) {
		return nil, fmt.Errorf("could not determine how to list installed packages")
	}
	switch {
		case utils.CheckLinuxCmdExists("rpm"):
		listPkgs = func()([]string, error) { o, err := exec.Command("rpm", "-qa", "--queryformat", "%{NAME}\n").Output(); return strings.Split(string(o), "\n"), err }
		case utils.CheckLinuxCmdExists("dpkg-query") && utils.CheckLinuxCmdExists("snap"):
		listPkgs = func()([]string, error) {
			var pkgs []string
			dpkgout, err := exec.Command("dpkg-query", "-W", "--showformat", "${Package}\n").Output();
			if err != nil {
				return nil, err
			}
			pkgs = append(pkgs, strings.Split(string(dpkgout), "\n")...)
			// Snap format name regexp source:
			// https://snapcraft.io/docs/the-snap-format
			// Technically incorrect as it won't match a single character package names
			// But nothing we're looking for has a single character name
			snapname := regexp.MustCompile("[a-z0-9][a-z0-9-]*[a-z0-9]")
			snapout, err := exec.Command("snap", "list").Output()
			if err != nil {
				return nil, err
			}
			for i, line := range strings.Split(string(snapout), "\n") {
				if i == 0 {
					continue // Skip header
				}
				if pkg := snapname.FindString(line); pkg != "" {
					pkgs = append(pkgs, pkg)
				}
			}
			return pkgs, nil
		}
		case utils.CheckLinuxCmdExists("dpkg-query"):
		listPkgs = func()([]string, error) { o, err := exec.Command("dpkg-query", "-W", "--showformat", "${Package}\n").Output(); return strings.Split(string(o), "\n"), err }
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

	installedPkgs, err := listPkgs()
	if err != nil {
		t.Errorf("Failed to execute list packages command: %v", err)
		return
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
