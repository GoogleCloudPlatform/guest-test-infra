//go:build cit
// +build cit

package imagevalidation

import (
	"testing"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var imageLicenseCodeMap = map[string]string{
	"centos-7":                    "1000207",
	"centos-stream":               "3197331720697687881",
	"rhel-7":                      "1000006",
	"rhel-8":                      "601259152637613565",
	"rhel-7-byos":                 "1492188837615955530",
	"rhel-8-byos":                 "8475125252192923229",
	"rhel-7-4-sap":                "5882583258875011738",
	"rhel-7-6-sap":                "8555687517154622919",
	"rhel-7-7-sap":                "8555687517154622919",
	"rhel-8-1-sap":                "1270685562947480748",
	"rhel-8-2-sap":                "1270685562947480748",
	"rhel-9-0-sap":                "8291906032809750558",
	"sles-12-sp5":                 "1000008",
	"sles-15-sp2":                 "5422776498422280384",
	"sles-12-sp3-sap":             "4079932016749305610",
	"sles-12-sp4-sap":             "4079932016749305610",
	"sles-12-sp5-sap":             "4079932016749305610",
	"sles-15-sap":                 "4764125400812555962",
	"sles-15-sp1-sap":             "4764125400812555962",
	"sles-15-sp2-sap":             "4764125400812555962",
	"debian-9-stretch":            "1000205",
	"debian-10-buster":            "5543610867827062957",
	"debian-11":                   "3853522013536123851",
	"ubuntu-1604-xenial":          "1000201",
	"ubuntu-1804-bionic":          "5926592092274602096",
	"ubuntu-2004-focal":           "2211838267635035815",
	"ubuntu-2010-groovy":          "769514169992655511",
	"ubuntu-2104-hirsute":         "7272665912576537111",
	"ubuntu-minimal-1604-xenial":  "1221576520422937469",
	"ubuntu-minimal-1804-bionic":  "5378856944553710442",
	"ubuntu-minimal-2004-focal":   "4650988716595113600",
	"ubuntu-minimal-2010-groovy":  "8143918599989015142",
	"ubuntu-minimal-2104-hirsute": "4947589846537827291",
}

func TestLinuxLicense(t *testing.T) {
	// Assume only one license exist in image
	licenseCode, err := utils.GetMetadata("licenses/0/id")
	if err != nil {
		t.Fatal("Failed to get license code metadata")
	}
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatal("Failed to get image metadata")
	}

	imageName, err := utils.ExtractBaseImageName(image)
	if err != nil {
		t.Fatal(err)
	}
	if code, found := imageLicenseCodeMap[imageName]; !found || code != licenseCode {
		t.Fatalf("Image %s has incorrect licenseCode %s", imageName, licenseCode)
	}
}
