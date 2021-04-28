package imagevalidation

import (
	"strings"
	"testing"
	"unicode"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var imageLicenseCodeMap = map[string]string{
	"centos-7":                    "1000207",
	"centos-8":                    "5731035067256925298",
	"centos-stream":               "3197331720697687881",
	"rhel-7":                      "1000006",
	"rhel-8":                      "601259152637613565",
	"rhel-7-byos":                 "1492188837615955530",
	"rhel-8-byos":                 "8475125252192923229",
	"suse-12-sp5":                 "1000008",
	"suse-15-sp2":                 "5422776498422280384",
	"suse-12-sp3-sap":             "4079932016749305610",
	"suse-12-sp4-sap":             "4079932016749305610",
	"suse-12-sp5-sap":             "4079932016749305610",
	"suse-15-sap":                 "4764125400812555962",
	"suse-15-sp1-sap":             "4764125400812555962",
	"suse-15-sp2-sap":             "4764125400812555962",
	"debian-9":                    "1000205",
	"debian-10":                   "5543610867827062957",
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
		t.Fatalf("Failed to get license code metadata")
	}
	image, err := utils.GetMetadata("image")
	if err != nil {
		t.Fatalf("Failed to get image metadata")
	}

	// Example: projects/rhel-cloud/global/images/rhel-8-v20210217
	splits := strings.Split(image, "/")
	imageName := splits[4]
	imagePrefix := strings.TrimRightFunc(imageName, func(r rune) bool {
		return unicode.IsDigit(r) || r == 'v' || r == '-'
	})

	if code, found := imageLicenseCodeMap[imagePrefix]; found == true && code == licenseCode {
		t.Logf("Image %s has licenseCode %s, %s", imageName, licenseCode)
	}
	t.Fatalf("Image has unkown licenseCode %s", licenseCode)
}
