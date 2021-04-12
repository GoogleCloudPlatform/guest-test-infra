package imagetest

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

var (
	client *storage.Client
)

const (
	testBinariesPath  = "/out"
	testWrapperPath   = testBinariesPath + "/wrapper"
	baseGCSPath       = "gs://gcp-guest-test-outputs/cloud_image_tests/"
	createVMsStepName = "create-vms"
)

// TestWorkflow defines a test workflow which creates at least one test VM.
type TestWorkflow struct {
	Image          string
	Name           string
	ShortImage     string
	destination    string
	skipped        bool
	skippedMessage string
	wf             *daisy.Workflow
}

func finalizeWorkflows(tests []*TestWorkflow, zone, project string) error {
	run := time.Now().Format(time.RFC3339)
	for _, ts := range tests {
		if ts.wf == nil {
			continue
		}
		ts.destination = fmt.Sprintf("%s/%s/%s/%s", baseGCSPath, run, ts.Name, ts.ShortImage)

		ts.wf.GCSPath = ts.destination

		ts.wf.DisableGCSLogging()
		ts.wf.DisableCloudLogging()
		ts.wf.DisableStdoutLogging()

		ts.wf.Zone = zone
		ts.wf.Project = project

		ts.wf.Sources["wrapper"] = testWrapperPath
		ts.wf.Sources["testpackage"] = fmt.Sprintf("%s/%s.test", testBinariesPath, ts.Name)

		copyGCSObject := daisy.CopyGCSObject{}
		copyGCSObject.Source = "${OUTSPATH}/" // Trailing slash apparently crucial.
		copyGCSObject.Destination = ts.destination + "/outs"
		copyGCSObjects := &daisy.CopyGCSObjects{copyGCSObject}
		copyStep, err := ts.wf.NewStep("copy-objects")
		if err != nil {
			return fmt.Errorf("failed to add copy-objects step to workflow %s: %v", ts.Name, err)
		}
		copyStep.CopyGCSObjects = copyGCSObjects

		// The "copy-objects" step depends on every wait step.
		for stepname, step := range ts.wf.Steps {
			if !strings.HasPrefix(stepname, "wait-") {
				continue
			}
			if err := ts.wf.AddDependency(copyStep, step); err != nil {
				return fmt.Errorf("failed to add copy-objects step dependency to workflow %s: %v", ts.Name, err)
			}
		}

	}
	return nil
}

type testResult struct {
	testWorkflow    *TestWorkflow
	skipped         bool
	workflowSuccess bool
	err             error
	results         []string
}

func getTestResults(ctx context.Context, ts *TestWorkflow) ([]string, error) {
	results := []string{}
	createVMsStep := ts.wf.Steps[createVMsStepName]
	for _, vm := range createVMsStep.CreateInstances.Instances {
		log.Printf("going to download %s", vm.Metadata["_test_results_url"])
		out, err := utils.DownloadGCSObject(ctx, client, vm.Metadata["_test_results_url"])
		if err != nil {
			return nil, fmt.Errorf("failed to get results for test %s vm %s: %v", ts.Name, vm.Name, err)
		}
		results = append(results, string(out))
	}

	return results, nil
}

// NewTestWorkflow returns a new TestWorkflow.
func NewTestWorkflow(name, image string) (*TestWorkflow, error) {
	t := &TestWorkflow{}
	t.Name = name
	t.Image = image

	parts := strings.Split(image, "/")
	t.ShortImage = parts[len(parts)-1]

	t.wf = daisy.New()
	t.wf.Name = strings.ReplaceAll(name, "_", "-")

	return t, nil
}

// PrintTests prints all test workflows.
func PrintTests(ctx context.Context, testWorkflows []*TestWorkflow, project, zone string) {
	finalizeWorkflows(testWorkflows, zone, project)
	for _, test := range testWorkflows {
		if test.wf == nil {
			continue
		}
		test.wf.Print(ctx)
	}
}

// ValidateTests validates all test workflows.
func ValidateTests(ctx context.Context, testWorkflows []*TestWorkflow, project, zone string) error {
	finalizeWorkflows(testWorkflows, zone, project)
	for _, test := range testWorkflows {
		if test.wf == nil {
			continue
		}
		if err := test.wf.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// RunTests runs all test workflows.
func RunTests(ctx context.Context, testWorkflows []*TestWorkflow, outPath, project, zone string, parallelCount int) {
	var err error
	client, err = storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to set up storage client: %v", err)
	}
	finalizeWorkflows(testWorkflows, zone, project)

	testResults := make(chan testResult, len(testWorkflows))
	testchan := make(chan *TestWorkflow, len(testWorkflows))

	var wg sync.WaitGroup
	for i := 0; i < parallelCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for test := range testchan {
				testResults <- runTestWorkflow(ctx, test)
			}
		}(i)
	}
	for _, ts := range testWorkflows {
		testchan <- ts
	}
	close(testchan)
	wg.Wait()

	var suites testSuites
	for i := 0; i < len(testWorkflows); i++ {
		suites.TestSuite = append(suites.TestSuite, parseResult(<-testResults))
	}

	bytes, err := xml.MarshalIndent(suites, "", "\t")
	if err != nil {
		fmt.Printf("failed to marshall result: %v\n", err)
		return
	}
	outFile, err := os.Create(outPath)
	if err != nil {
		fmt.Printf("failed to create output file: %v\n", err)
		return
	}
	defer outFile.Close()

	outFile.Write(bytes)
	outFile.Write([]byte{'\n'})

	return
}

func runTestWorkflow(ctx context.Context, test *TestWorkflow) testResult {
	var res testResult
	res.testWorkflow = test
	if test.skipped {
		res.skipped = true
		res.err = fmt.Errorf("Test suite was skipped")
		return res
	}
	fmt.Printf("runTestWorkflow: running %s on %s (ID %s)\n", test.Name, test.ShortImage, test.wf.ID())
	if err := test.wf.Run(ctx); err != nil {
		res.err = err
		return res
	}
	results, err := getTestResults(ctx, test)
	if err != nil {
		res.err = err
		return res
	}
	res.results = results
	res.workflowSuccess = true

	return res
}

// gets result struct and converts to a jUnit TestSuite
func parseResult(res testResult) *testSuite {
	ret := &testSuite{}

	switch {
	case res.skipped:
		ret.Tests = 1
		ret.Skipped = 1
	case res.workflowSuccess:
		// Workflow completed without error. Only in this case do we try to parse the result.
		ret = convertToTestSuite(res.results)
	default:
		ret.Tests = 1
		ret.Errors = 1
		if res.err != nil {
			ret.SystemErr = res.err.Error()
		} else {
			ret.SystemErr = "Unknown status"
		}
	}

	ret.Name = fmt.Sprintf("%s-%s", res.testWorkflow.Name, res.testWorkflow.ShortImage)
	return ret
}
