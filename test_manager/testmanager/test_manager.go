package testmanager

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/test_utils"
	junitFormatter "github.com/jstemmer/go-junit-report/formatter"
)

var (
	client *storage.Client
)

const (
	testBinariesPath = "/tmp/out"
	testWrapperPath  = testBinariesPath + "/test_wrapper"
)

// TestWorkflow defines a test workflow which creates at least one test VM.
type TestWorkflow struct {
	wf          *daisy.Workflow
	Name        string
	Image       string
	skipped     bool
	destination string
}

// Disable disables a workflow.
func (t *TestWorkflow) Disable() {
	t.wf = nil
}

func finalizeWorkflows(tests []*TestWorkflow, zone, project string) {
	for _, ts := range tests {
		if ts.wf == nil {
			continue
		}
		ts.destination = fmt.Sprintf("gs://liamh-export/new_test_manager_testing/0001/%s-%s/junit.xml", ts.wf.Name, ts.wf.ID())

		ts.wf.DisableGCSLogging()
		ts.wf.DisableCloudLogging()
		ts.wf.DisableStdoutLogging()

		ts.wf.Zone = zone
		ts.wf.Project = project

		ts.wf.Sources["startup"] = testWrapperPath
		ts.wf.Sources["testbinary"] = fmt.Sprintf("%s/%s.test", testBinariesPath, ts.Name)

		copyStep := ts.wf.Steps["copy-objects"]

		// Two issues with manipulating this step. First, it is a
		// typedef that removes the slice notation, so we have to cast
		// it back in order to index it.
		copyObject := []daisy.CopyGCSObject(*copyStep.CopyGCSObjects)[0]
		copyObject.Destination = ts.destination

		// Second, it is not a pointer, so we can't modify it in place.
		// Instead, we overwrite the struct with a new step with our
		// modified copy of the config.
		copyStep.CopyGCSObjects = &daisy.CopyGCSObjects{copyObject}

		// Add metadata to each VM.
		for _, vm := range ts.wf.Steps["create-vms"].CreateInstances.Instances {
			if vm.Metadata == nil {
				vm.Metadata = make(map[string]string)
			}
			vm.Metadata["_test_binary_url"] = "${SOURCESPATH}/testbinary"
		}
	}
}

type testResult struct {
	testWorkflow                    *TestWorkflow
	Skipped, FailedSetup            bool
	WorkflowFailed, WorkflowSuccess bool
	Result                          string
}

func getTestResults(ctx context.Context, ts *TestWorkflow) (string, error) {
	junit, err := test_utils.DownloadGCSObject(ctx, client, ts.destination)
	if err != nil {
		return "", err
	}

	return string(junit), nil
}

func runTestWorkflow(ctx context.Context, test *TestWorkflow) testResult {
	var res testResult
	res.testWorkflow = test
	if test.skipped {
		res.Skipped = true
		return res
	}
	if test.wf == nil {
		res.FailedSetup = true
		return res
	}
	// TODO: remove this debug line
	fmt.Printf("runTestWorkflow: running %s on %s (ID %s)\n", test.Name, test.Image, test.wf.ID())
	if err := test.wf.Run(ctx); err != nil {
		res.WorkflowFailed = true
		res.Result = err.Error()
		return res
	}
	results, err := getTestResults(ctx, test)
	if err != nil {
		res.WorkflowFailed = true
		res.Result = err.Error()
		return res
	}
	res.WorkflowSuccess = true
	res.Result = results

	return res
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

	var suites junitFormatter.JUnitTestSuites
	for i := 0; i < len(testWorkflows); i++ {
		suites.Suites = append(suites.Suites, parseResult(<-testResults))
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

func parseResult(res testResult) junitFormatter.JUnitTestSuite {
	var ret junitFormatter.JUnitTestSuite
	//ret.Name = res.testWorkflow.Name

	switch {
	case res.FailedSetup:
		fmt.Printf("test %s on %s failed during setup and was disabled\n", res.testWorkflow.Name, res.testWorkflow.Image)
		ret.Failures = 1
		return ret
	case res.Skipped:
		fmt.Printf("test %s on %s was skipped\n", res.testWorkflow.Name, res.testWorkflow.Image)
		return ret
	case res.WorkflowFailed:
		// We didn't run it, it timed out, it didn't upload a result, etc.
		fmt.Printf("test %s on %s workflow failed: %s\n", res.testWorkflow.Name, res.testWorkflow.Image, res.Result)
		return ret
	case res.WorkflowSuccess:
		// Workflow completed without error. Only in this case do we try to parse the result.
		fmt.Printf("test %s on %s workflow completed without error\n", res.testWorkflow.Name, res.testWorkflow.Image)
		var suites junitFormatter.JUnitTestSuites
		if err := xml.Unmarshal([]byte(res.Result), &suites); err != nil {
			//fmt.Printf("Failed to unmarshal junit results: %v\n", err)
			failure := &junitFormatter.JUnitFailure{}
			failure.Contents = "Test setup failed"
			failure.Message = res.Result
			testcase := junitFormatter.JUnitTestCase{}
			testcase.Name = res.testWorkflow.Name + "-Setup"
			testcase.Failure = failure
			ret.TestCases = append(ret.TestCases, testcase)
			ret.Failures = 1
			ret.Tests = 1
			return ret
		}
		suite := suites.Suites[0]
		fmt.Printf("%+v\n", suite)
		suite.Name = res.testWorkflow.Name
		return suite
	default:
		fmt.Printf("test %s on %s has unknown status\n", res.testWorkflow.Name, res.testWorkflow.Image)
	}

	return ret
}
