package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest"
	imagevalidation "github.com/GoogleCloudPlatform/guest-test-infra/imagetest/test_suites/image_validation"
)

var (
	project       = flag.String("project", "", "project to be used for tests")
	zone          = flag.String("zone", "", "zone to be used for tests")
	printwf       = flag.Bool("print", false, "print out the parsed test workflows and exit")
	validate      = flag.Bool("validate", false, "validate all the test workflows and exit")
	outPath       = flag.String("out_path", "junit.xml", "junit xml path")
	images        = flag.String("images", "", "comma separated list of images to test")
	parallelCount = flag.Int("parallel_count", 5, "TestParallelCount")
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

	// Setup tests.
	testPackages := []struct {
		name      string
		setupFunc func(*imagetest.TestWorkflow) error
	}{
		{
			imagevalidation.Name,
			imagevalidation.TestSetup,
		},
	}

	var testWorkflows []*imagetest.TestWorkflow
	for _, testPackage := range testPackages {
		for _, image := range strings.Split(*images, ",") {
			test, err := imagetest.NewTestWorkflow(testPackage.name, image)
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

	out, err := imagetest.RunTests(ctx, testWorkflows, *outPath, *project, *zone, *parallelCount)
	if err != nil {
		log.Fatalf("Failed to run tests: %v", err)
	}
	fmt.Printf("%s\n", out)
}
