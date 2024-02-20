package licensevalidation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

var imageSuffixRe = regexp.MustCompile(`-(arm|amd|x86_)64$`)
var sqlWindowsVersionRe = regexp.MustCompile("windows-[0-9]{4}-dc")
var sqlVersionRe = regexp.MustCompile("sql-[0-9]{4}-(express|enterprise|standard|web)")

// Name is the name of the test package. It must match the directory name.
var Name = "licensevalidation"

// TestSetup sets up the test workflow.
func TestSetup(t *imagetest.TestWorkflow) error {
	licensetests := "TestLicenses"
	if utils.HasFeature(t.Image, "WINDOWS") {
		licensetests += "|TestWindowsActivationStatus"
	}
	vm1, err := t.CreateTestVM("licensevm")
	if err != nil {
		return err
	}
	rlicenses, err := requiredLicenseList(t.Image)
	if err != nil {
		return err
	}
	vm1.AddMetadata("expected-licenses", rollStringToString(rlicenses))
	vm1.AddMetadata("actual-licenses", rollStringToString(t.Image.Licenses))
	vm1.AddMetadata("expected-license-codes", rollInt64ToString(t.Image.LicenseCodes))
	if err != nil {
		return err
	}
	vm1.RunTests(licensetests)
	return nil
}

func rollStringToString(list []string) string {
	var result string
	for i, item := range list {
		if i != 0 {
			result += ","
		}
		result += fmt.Sprintf("%s", item)
	}
	return result
}

func rollInt64ToString(list []int64) string {
	var result string
	for i, item := range list {
		if i != 0 {
			result += ","
		}
		result += fmt.Sprintf("%d", item)
	}
	return result
}

// generate a list of license URLs we should expect to see on the image from the Name and Family properties
func requiredLicenseList(image *compute.Image) ([]string, error) {
	licenseURLTmpl := "https://www.googleapis.com/compute/v1/projects/%s/global/licenses/%s"
	transform := func() {}
	var requiredLicenses []string
	var project string
	switch {
	case strings.Contains(image.Name, "debian"):
		project = "debian-cloud"
		transform = func() {
			// Rightmost dash separated segment with only [a-z] chars should be the codename
			var codename string
			segments := strings.Split(image.Name, "-")
			for i := len(segments) - 1; i >= 0; i-- {
				if len(regexp.MustCompile("[a-z]+").FindString(segments[i])) == len(segments[i]) {
					codename = segments[i]
					break
				}
			}
			for i := range requiredLicenses {
				requiredLicenses[i] += "-" + codename
			}
		}
	case strings.Contains(image.Name, "rhel") && strings.Contains(image.Name, "sap"):
		project = "rhel-sap-cloud"
		transform = func() {
			newSuffix := "-sap"
			if strings.Contains(image.Name, "byos") {
				newSuffix += "-byos"
			}
			rhelSapVersionRe := regexp.MustCompile("-[0-9]+-sap-(ha|byos)$")
			requiredLicenses[len(requiredLicenses)-1] = rhelSapVersionRe.ReplaceAllString(requiredLicenses[len(requiredLicenses)-1], newSuffix)
		}
	case strings.Contains(image.Name, "rhel"):
		project = "rhel-cloud"
		transform = func() {
			if !strings.Contains(image.Name, "byos") {
				requiredLicenses[len(requiredLicenses)-1] += "-server"
			}
			requiredLicenses[len(requiredLicenses)-1] = strings.ReplaceAll(requiredLicenses[len(requiredLicenses)-1], "-c3m", "")
		}
	case strings.Contains(image.Name, "centos"):
		project = "centos-cloud"
		transform = func() {
			if image.Family == "centos-stream-8" {
				// centos-stream-8 doesn't include -8
				requiredLicenses[len(requiredLicenses)-1] = requiredLicenses[len(requiredLicenses)-1][:len(requiredLicenses[len(requiredLicenses)-1])-2]
			}
		}
	case strings.Contains(image.Name, "rocky-linux"):
		project = "rocky-linux-cloud"
	case strings.Contains(image.Name, "almalinux"):
		project = "almalinux-cloud"
	case strings.Contains(image.Name, "opensuse"):
		project = "opensuse-cloud"
		transform = func() { requiredLicenses[len(requiredLicenses)-1] += "-42" } // Quirk of opensuse licensing. This suffix will not need to be updated with version changes.
	case strings.Contains(image.Name, "sles") && strings.Contains(image.Name, "sap"):
		project = "suse-sap-cloud"
	case strings.Contains(image.Name, "sles"):
		project = "suse-cloud"
	case strings.Contains(image.Name, "ubuntu-pro"):
		project = "ubuntu-os-pro-cloud"
	case strings.Contains(image.Name, "ubuntu"):
		project = "ubuntu-os-cloud"
	case strings.Contains(image.Name, "windows") && strings.Contains(image.Name, "sql"):
		project = "windows-cloud"
		transform = func() {
			requiredLicenses = []string{
				fmt.Sprintf(licenseURLTmpl, "windows-sql-cloud", strings.Replace(sqlVersionRe.FindString(image.Name), "sql-", "sql-server-", -1)),
				fmt.Sprintf(licenseURLTmpl, project, strings.Replace(sqlWindowsVersionRe.FindString(image.Name), "windows-", "windows-server-", -1)),
			}
		}
	case strings.Contains(image.Name, "windows"):
		project = "windows-cloud"
		transform = func() {
			requiredLicenses = []string{fmt.Sprintf(licenseURLTmpl, project, "windows-server-"+regexp.MustCompile("[0-9]{4}").FindString(image.Family)+"-dc")}
			if strings.Contains(image.Name, "core") {
				requiredLicenses = append(requiredLicenses, fmt.Sprintf(licenseURLTmpl, project, "windows-server-core"))
			}
		}
	default:
		return nil, fmt.Errorf("Not sure what project to look for licenses from for %s", image.Name)
	}

	requiredLicenses = append(requiredLicenses, fmt.Sprintf(licenseURLTmpl, project, imageSuffixRe.ReplaceAllString(image.Family, "")))
	transform()

	return requiredLicenses, nil
}
