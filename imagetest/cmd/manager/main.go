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

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/disk"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/imageboot"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/imagevalidation"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/metadata"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/network"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/networkperf"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/oslogin"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/security"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/sql"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/ssh"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/storageperf"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/windows"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/windowscontainers"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/windowsimagevalidation"
)

var (
	project       = flag.String("project", "", "project to use for test runner")
	testProjects  = flag.String("test_projects", "", "comma separated list of projects to be used for tests. defaults to the test runner project")
	zone          = flag.String("zone", "", "zone to be used for tests")
	printwf       = flag.Bool("print", false, "print out the parsed test workflows and exit")
	validate      = flag.Bool("validate", false, "validate all the test workflows and exit")
	outPath       = flag.String("out_path", "junit.xml", "junit xml path")
	gcsPath       = flag.String("gcs_path", "", "GCS Path for Daisy working directory")
	images        = flag.String("images", "", "comma separated list of images to test")
	timeout       = flag.String("timeout", "45m", "timeout for the test suite")
	parallelCount = flag.Int("parallel_count", 5, "TestParallelCount")
	filter        = flag.String("filter", "", "only run tests matching filter")
	exclude       = flag.String("exclude", "", "skip tests matching filter")
	machineType   = flag.String("machine_type", "", "machine type to use for test instances")
)

var (
	projectMap = map[string]string{
		"almalinux":     "almalinux-cloud",
		"centos":        "centos-cloud",
		"cos":           "cos-cloud",
		"debian":        "debian-cloud",
		"fedora-cloud":  "fedora-cloud",
		"fedora-coreos": "fedora-coreos-cloud",
		"opensuse":      "opensuse-cloud",
		"rhel":          "rhel-cloud",
		"rhel-sap":      "rhel-sap-cloud",
		"rocky-linux":   "rocky-linux-cloud",
		"sles":          "suse-cloud",
		"sles-sap":      "suse-sap-cloud",
		"sql-":          "windows-sql-cloud",
		"ubuntu":        "ubuntu-os-cloud",
		"ubuntu-pro":    "ubuntu-os-pro-cloud",
		"windows":       "windows-cloud",
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
	var testProjectsReal []string
	if *testProjects == "" {
		testProjectsReal = append(testProjectsReal, *project)
	} else {
		testProjectsReal = strings.Split(*testProjects, ",")
	}

	log.Printf("Running in project %s zone %s. Tests will run in projects: %s", *project, *zone, testProjectsReal)
	if *gcsPath != "" {
		log.Printf("gcs_path set to %s", *gcsPath)
	}

	var filterRegex *regexp.Regexp
	if *filter != "" {
		var err error
		filterRegex, err = regexp.Compile(*filter)
		if err != nil {
			log.Fatal("-filter flag not valid:", err)
		}
		log.Printf("using -filter %s", *filter)
	}

	var excludeRegex *regexp.Regexp
	if *exclude != "" {
		var err error
		excludeRegex, err = regexp.Compile(*exclude)
		if err != nil {
			log.Fatal("-exclude flag not valid:", err)
		}
		log.Printf("using -exclude %s", *exclude)
	}

	// Setup tests.
	testPackages := []struct {
		name      string
		setupFunc func(*imagetest.TestWorkflow) error
	}{
		{
			networkperf.Name,
			networkperf.TestSetup,
		},
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
			storageperf.Name,
			storageperf.TestSetup,
		},
		{
			ssh.Name,
			ssh.TestSetup,
		},
		{
			sql.Name,
			sql.TestSetup,
		},
		{
			metadata.Name,
			metadata.TestSetup,
		},
		{
			oslogin.Name,
			oslogin.TestSetup,
		},
		{
			windows.Name,
			windows.TestSetup,
		},
		{
			windowscontainers.Name,
			windowscontainers.TestSetup,
		},
		{
			windowsimagevalidation.Name,
			windowsimagevalidation.TestSetup,
		},
	}

	var testWorkflows []*imagetest.TestWorkflow
	for _, testPackage := range testPackages {
		if filterRegex != nil && !filterRegex.MatchString(testPackage.name) {
			continue
		}
		if excludeRegex != nil && excludeRegex.MatchString(testPackage.name) {
			continue
		}
		for _, image := range strings.Split(*images, ",") {
			if !strings.Contains(image, "/") {
				// Find the project of the image.
				project := ""
				for k := range projectMap {
					if strings.Contains(k, "sap") {
						// sap follows a slightly different naming convention.
						imageName := strings.Split(k, "-")[0]
						if strings.HasPrefix(image, imageName) && strings.Contains(image, "sap") {
							project = projectMap[k]
							break
						}
					}
					if strings.HasPrefix(image, k) {
						project = projectMap[k]
						break
					}
				}
				if project == "" {
					log.Fatalf("unknown image %s", image)
				}

				// Check whether the image is an image family or a specific image version.
				isMatch, err := regexp.MatchString(".*v([0-9]+)", image)
				if err != nil {
					log.Fatalf("failed regex: %v", err)
				}
				if isMatch {
					image = fmt.Sprintf("projects/%s/global/images/%s", project, image)
				} else {
					image = fmt.Sprintf("projects/%s/global/images/family/%s", project, image)
				}
			}

			log.Printf("Add test workflow for test %s on image %s", testPackage.name, image)
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

	if len(testWorkflows) == 0 {
		log.Fatalf("No workflows to run!")
	}

	log.Println("Done with setup")

	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to set up storage client: %v", err)
	}

	if *printwf {
		imagetest.PrintTests(ctx, client, testWorkflows, *project, *zone, *gcsPath, *machineType)
		return
	}

	if *validate {
		if err := imagetest.ValidateTests(ctx, client, testWorkflows, *project, *zone, *gcsPath, *machineType); err != nil {
			log.Printf("Validate failed: %v\n", err)
		}
		return
	}

	suites, err := imagetest.RunTests(ctx, client, testWorkflows, *project, *zone, *gcsPath, *machineType, *parallelCount, testProjectsReal)
	if err != nil {
		log.Fatalf("Failed to run tests: %v", err)
	}

	bytes, err := xml.MarshalIndent(suites, "", "\t")
	if err != nil {
		log.Fatalf("failed to marshall result: %v", err)
	}
	var outFile *os.File
	if artifacts := os.Getenv("ARTIFACTS"); artifacts != "" {
		outFile, err = os.Create(artifacts + "/junit.xml")
	} else {
		outFile, err = os.Create(*outPath)
	}
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
