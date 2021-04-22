package imagevalidation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
)

var licenseSpecPairs = map[string]string{
	"centos-7":    "centos-7",
	"centos-8":    "centos-8",
	"rhel-7":      "rhel-7-server",
	"rhel-8":      "rhel-8-server",
	"rhel-7-byos": "rhel-7-byos",
	"rhel-8-byos": "rhel-8-byos",
	"debian-9":    "debian-9-stretch",
	"debian-10":   "debian-10-stretch",
}

func TestLinuxLicense(t *testing.T) {
	licenseCode, err := utils.GetMetadata("licenses/0/id")
	if err != nil {
		t.Fatalf("Failed to get license code metadata")
	}
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Failed to get image metadata")
	}

	// Example: projects/rhel-cloud/global/images/rhel-8-v20210217
	splits := strings.Split(image, "/")
	project, imageName := splits[1], splits[4]
	imagePrefix := strings.TrimRightFunc(imageName, func(r rune) bool {
		return unicode.IsDigit(r) || r == 'v' || r == '-'
	})

	licenses, err := getLicenseByLicenseCode(project, licenseCode)
	if err != nil {
		t.Fatal(err)
	}

	for _, license := range licenses {
		if licenseSpecPairs[imagePrefix] == license.Name {
			t.Logf("Image has licenseCode %s, %s", license.Name, licenseCode)
		}
	}
	t.Fatalf("Image has unkown licenseCode %s", licenseCode)
}

func getLicenseByLicenseCode(project string, licenseCode string) ([]*compute.License, error) {
	ctx := context.Background()
	computeService, err := compute.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get service %s", err)
	}

	response, err := computeService.Licenses.List(project).Filter(fmt.Sprintf("licenseCode = %s", licenseCode)).Do()
	if err != nil {
		return nil, fmt.Errorf("Failed to get licenseCode %s from project %s", licenseCode, project)
	}
	return response.Items, nil
}
