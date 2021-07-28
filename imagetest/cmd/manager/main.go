package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	artifactregistry "github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/artifact_registry"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/disk"
	imageboot "github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/image_boot"
	imagevalidation "github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/image_validation"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/metadata"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/network"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/security"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/ssh"
)

var (
	project       = flag.String("project", "", "project to be used for tests")
	zone          = flag.String("zone", "", "zone to be used for tests")
	printwf       = flag.Bool("print", false, "print out the parsed test workflows and exit")
	validate      = flag.Bool("validate", false, "validate all the test workflows and exit")
	outPath       = flag.String("out_path", "junit.xml", "junit xml path")
	images        = flag.String("images", "", "comma separated list of images to test")
	timeout       = flag.String("timeout", "30m", "timeout for the test suite")
	parallelCount = flag.Int("parallel_count", 5, "TestParallelCount")
	filter        = flag.String("filter", "", "only run tests matching filter")
)

var (
	imageMap = map[string]string{
		"centos-7":                "projects/centos-cloud/global/images/family/centos-7",
		"centos-8":                "projects/centos-cloud/global/images/family/centos-8",
		"centos-stream-8":         "projects/centos-cloud/global/images/family/centos-stream-8",
		"cos-77-lts":              "projects/cos-cloud/global/images/family/cos-77-lts",
		"cos-81-lts":              "projects/cos-cloud/global/images/family/cos-81-lts",
		"cos-85-lts":              "projects/cos-cloud/global/images/family/cos-85-lts",
		"cos-89-lts":              "projects/cos-cloud/global/images/family/cos-89-lts",
		"cos-beta":                "projects/cos-cloud/global/images/family/cos-beta",
		"cos-dev":                 "projects/cos-cloud/global/images/family/cos-dev",
		"cos-stable":              "projects/cos-cloud/global/images/family/cos-stable",
		"debian-10":               "projects/debian-cloud/global/images/family/debian-10",
		"debian-9":                "projects/debian-cloud/global/images/family/debian-9",
		"fedora-coreos-next":      "projects/fedora-coreos-cloud/global/images/family/fedora-coreos-next",
		"fedora-coreos-stable":    "projects/fedora-coreos-cloud/global/images/family/fedora-coreos-stable",
		"fedora-coreos-testing":   "projects/fedora-coreos-cloud/global/images/family/fedora-coreos-testing",
		"rhel-7":                  "projects/rhel-cloud/global/images/family/rhel-7",
		"rhel-7-4-sap":            "projects/rhel-sap-cloud/global/images/family/rhel-7-4-sap",
		"rhel-7-6-sap-ha":         "projects/rhel-sap-cloud/global/images/family/rhel-7-6-sap-ha",
		"rhel-7-7-sap-ha":         "projects/rhel-sap-cloud/global/images/family/rhel-7-7-sap-ha",
		"rhel-8":                  "projects/rhel-cloud/global/images/family/rhel-8",
		"rhel-8-1-sap-ha":         "projects/rhel-sap-cloud/global/images/family/rhel-8-1-sap-ha",
		"rhel-8-2-sap-ha":         "projects/rhel-sap-cloud/global/images/family/rhel-8-2-sap-ha",
		"rocky-linux-8":           "projects/rocky-linux-cloud/global/images/family/rocky-linux-8",
		"sles-12":                 "projects/suse-cloud/global/images/family/sles-12",
		"sles-12-sp3-sap":         "projects/suse-sap-cloud/global/images/family/sles-12-sp3-sap",
		"sles-12-sp4-sap":         "projects/suse-sap-cloud/global/images/family/sles-12-sp4-sap",
		"sles-12-sp5-sap":         "projects/suse-sap-cloud/global/images/family/sles-12-sp5-sap",
		"sles-15":                 "projects/suse-cloud/global/images/family/sles-15",
		"sles-15-sap":             "projects/suse-sap-cloud/global/images/family/sles-15-sap",
		"sles-15-sp1-sap":         "projects/suse-sap-cloud/global/images/family/sles-15-sp1-sap",
		"sles-15-sp2-sap":         "projects/suse-sap-cloud/global/images/family/sles-15-sp2-sap",
		"ubuntu-1604-lts":         "projects/ubuntu-os-cloud/global/images/family/ubuntu-1604-lts",
		"ubuntu-1804-lts":         "projects/ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
		"ubuntu-2004-lts":         "projects/ubuntu-os-cloud/global/images/family/ubuntu-2004-lts",
		"ubuntu-2010":             "projects/ubuntu-os-cloud/global/images/family/ubuntu-2010",
		"ubuntu-2104":             "projects/ubuntu-os-cloud/global/images/family/ubuntu-2104",
		"ubuntu-minimal-1604-lts": "projects/ubuntu-os-cloud/global/images/family/ubuntu-minimal-1604-lts",
		"ubuntu-minimal-1804-lts": "projects/ubuntu-os-cloud/global/images/family/ubuntu-minimal-1804-lts",
		"ubuntu-minimal-2004-lts": "projects/ubuntu-os-cloud/global/images/family/ubuntu-minimal-2004-lts",
		"ubuntu-minimal-2010":     "projects/ubuntu-os-cloud/global/images/family/ubuntu-minimal-2010",
		"ubuntu-minimal-2104":     "projects/ubuntu-os-cloud/global/images/family/ubuntu-minimal-2104",
		"ubuntu-pro-1604-lts":     "projects/ubuntu-os-pro-cloud/global/images/family/ubuntu-pro-1604-lts",
		"ubuntu-pro-1804-lts":     "projects/ubuntu-os-pro-cloud/global/images/family/ubuntu-pro-1804-lts",
		"ubuntu-pro-2004-lts":     "projects/ubuntu-os-pro-cloud/global/images/family/ubuntu-pro-2004-lts",
	}
)

type logWriter struct {
	log *log.Logger
}

func (l *logWriter) Write(b []byte) (int, error) {
	l.log.Print(string(b))
	return len(b), nil
}

func main() {
	flag.Parse()
	if *project == "" || *zone == "" || *images == "" {
		log.Fatal("Must provide project, zone and images arguments")
		return
	}

	var regex *regexp.Regexp
	if *filter != "" {
		var err error
		regex, err = regexp.Compile(*filter)
		if err != nil {
			log.Fatal("-filter flag not valid:", err)
		}
	}

	// Setup tests.
	testPackages := []struct {
		name      string
		setupFunc func(*imagetest.TestWorkflow) error
	}{
		{
			imagevalidation.Name,
			imagevalidation.TestSetup,
		},
		{
			imageboot.Name,
			imageboot.TestSetup,
		},
		{
			network.Name,
			network.TestSetup,
		},
		{
			security.Name,
			security.TestSetup,
		},
		{
			disk.Name,
			disk.TestSetup,
		},
		{
			ssh.Name,
			ssh.TestSetup,
		},
		{
			metadata.Name,
			metadata.TestSetup,
		},
		{
			artifactregistry.Name,
			artifactregistry.TestSetup,
		},
	}

	var testWorkflows []*imagetest.TestWorkflow
	for _, testPackage := range testPackages {
		if regex != nil && !regex.MatchString(testPackage.name) {
			continue
		}
		for _, image := range strings.Split(*images, ",") {
			if !strings.Contains(image, "/") {
				fullimage, ok := imageMap[image]
				if !ok {
					log.Fatalf("unknown image %s", image)
				}
				image = fullimage
			}

			test, err := imagetest.NewTestWorkflow(testPackage.name, image, *timeout)
			if err != nil {
				log.Fatalf("Failed to create test workflow: %v", err)
			}
			testWorkflows = append(testWorkflows, test)
			if err := testPackage.setupFunc(test); err != nil {
				log.Fatalf("%s.TestSetup for %s failed: %v", testPackage.name, image, err)
			}
		}
	}

	log.Println("imagetest: Done with setup")

	ctx := context.Background()

	if *printwf {
		imagetest.PrintTests(ctx, testWorkflows, *project, *zone)
		return
	}

	if *validate {
		if err := imagetest.ValidateTests(ctx, testWorkflows, *project, *zone); err != nil {
			log.Printf("Validate failed: %v\n", err)
		}
		return
	}

	suites, err := imagetest.RunTests(ctx, testWorkflows, *project, *zone, *parallelCount)
	if err != nil {
		log.Fatalf("Failed to run tests: %v", err)
	}

	bytes, err := xml.MarshalIndent(suites, "", "\t")
	if err != nil {
		log.Fatalf("failed to marshall result: %v", err)
	}
	outFile, err := os.Create(*outPath)
	if err != nil {
		log.Fatalf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	outFile.Write(bytes)
	outFile.Write([]byte{'\n'})
	fmt.Printf("%s\n", bytes)

	if suites.Errors != 0 || suites.Failures != 0 {
		log.Fatalf("test suite has error or failure")
	}
}
