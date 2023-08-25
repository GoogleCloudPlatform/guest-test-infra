// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package imagetest

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	daisy "github.com/GoogleCloudPlatform/compute-daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
)

var (
	client *storage.Client
)

const (
	// PdStandard disktype string
	PdStandard = "pd-standard"
	// PdSsd disktype string
	PdSsd = "pd-ssd"
	// PdBalanced disktype string
	PdBalanced = "pd-balanced"
	// PdExtreme disktype string
	PdExtreme = "pd-extreme"
	// HyperdiskExtreme disktype string
	HyperdiskExtreme = "hyperdisk-extreme"
	// HyperdiskThroughput disktype string
	HyperdiskThroughput = "hyperdisk-throughput"
	// HyperdiskBalanced disktype string
	HyperdiskBalanced = "hyperdisk-balanced"

	testWrapperPath        = "/wrapper"
	testWrapperPathWindows = "/wrapp"
)

// TestWorkflow defines a test workflow which creates at least one test VM.
type TestWorkflow struct {
	Name string
	// Image will be the partial URL of a GCE image.
	Image string
	// ShortImage will be only the final component of Image, used for naming.
	ShortImage string
	// destination for workflow outputs in GCS.
	gcsPath        string
	skipped        bool
	skippedMessage string
	wf             *daisy.Workflow
	// Global counter for all daisy steps on all VMs. This is an interim solution in order to prevent step-name collisions.
	counter int
	// Does this test require exclusive project
	lockProject bool
}

func (t *TestWorkflow) appendCreateVMStep(disks []*compute.Disk, instanceParams map[string]string) (*daisy.Step, *daisy.Instance, error) {
	if len(disks) == 0 || disks[0].Name == "" {
		return nil, nil, fmt.Errorf("failed to create VM from empty boot disk")
	}
	// The boot disk is the first disk, and the VM name comes from that
	name := disks[0].Name

	var suffix string
	if strings.Contains(t.Image, "windows") {
		suffix = ".exe"
	}

	instance := &daisy.Instance{}
	instance.StartupScript = fmt.Sprintf("wrapper%s", suffix)
	instance.Name = name
	instance.Scopes = append(instance.Scopes, "https://www.googleapis.com/auth/devstorage.read_write")
	hostname, foundKey := instanceParams["hostname"]
	if !foundKey {
		hostname = ""
	}
	if hostname != "" && name != hostname {
		instance.Hostname = hostname
	}

	if machineType, foundKey := instanceParams["machineType"]; foundKey {
		instance.MachineType = machineType
	}

	for _, disk := range disks {
		instance.Disks = append(instance.Disks, &compute.AttachedDisk{Source: disk.Name})
	}

	instance.Metadata = make(map[string]string)
	instance.Metadata["_test_vmname"] = name
	instance.Metadata["_test_package_url"] = "${SOURCESPATH}/testpackage"
	instance.Metadata["_test_results_url"] = fmt.Sprintf("${OUTSPATH}/%s.txt", name)
	instance.Metadata["_test_package_name"] = fmt.Sprintf("image_test%s", suffix)

	createInstances := &daisy.CreateInstances{}
	createInstances.Instances = append(createInstances.Instances, instance)

	createVMStep, ok := t.wf.Steps[createVMsStepName]
	if ok {
		// append to existing step.
		createVMStep.CreateInstances.Instances = append(createVMStep.CreateInstances.Instances, instance)
	} else {
		var err error
		createVMStep, err = t.wf.NewStep(createVMsStepName)
		if err != nil {
			return nil, nil, err
		}
		createVMStep.CreateInstances = createInstances
	}

	return createVMStep, instance, nil
}

// appendCreateDisksStep should be called for creating the boot disk, or first disk in a VM.
func (t *TestWorkflow) appendCreateDisksStep(diskParams *compute.Disk) (*daisy.Step, error) {
	if diskParams == nil || diskParams.Name == "" {
		return nil, fmt.Errorf("failed to create disk with empty parameters")
	}
	bootdisk := &daisy.Disk{}
	bootdisk.Name = diskParams.Name
	bootdisk.SourceImage = t.Image
	bootdisk.Type = diskParams.Type

	createDisks := &daisy.CreateDisks{bootdisk}

	createDisksStep, ok := t.wf.Steps[createDisksStepName]
	if ok {
		// append to existing step.
		*createDisksStep.CreateDisks = append(*createDisksStep.CreateDisks, bootdisk)
	} else {
		var err error
		createDisksStep, err = t.wf.NewStep(createDisksStepName)
		if err != nil {
			return nil, err
		}
		createDisksStep.CreateDisks = createDisks
	}

	return createDisksStep, nil
}

// appendCreateMountDisksStep should be called for any disk which is not the vm boot disk.
func (t *TestWorkflow) appendCreateMountDisksStep(diskParams *compute.Disk) (*daisy.Step, error) {
	if diskParams == nil || diskParams.Name == "" {
		return nil, fmt.Errorf("failed to create disk with empty parameters")
	}
	mountdisk := &daisy.Disk{}
	mountdisk.Name = diskParams.Name
	mountdisk.Type = diskParams.Type
	if diskParams.SizeGb == 0 {
		return nil, fmt.Errorf("failed to create mount disk with no SizeGb parameter")
	}
	mountdisk.SizeGb = strconv.FormatInt(diskParams.SizeGb, 10)

	createDisks := &daisy.CreateDisks{mountdisk}

	createDisksStep, ok := t.wf.Steps[createDisksStepName]
	if ok {
		// append to existing step.
		*createDisksStep.CreateDisks = append(*createDisksStep.CreateDisks, mountdisk)
	} else {
		var err error
		createDisksStep, err = t.wf.NewStep(createDisksStepName)
		if err != nil {
			return nil, err
		}
		createDisksStep.CreateDisks = createDisks
	}

	return createDisksStep, nil
}

func (t *TestWorkflow) addWaitStep(stepname, vmname string, stopped bool) (*daisy.Step, error) {
	serialOutput := &daisy.SerialOutput{}
	serialOutput.Port = 1
	serialOutput.SuccessMatch = successMatch

	instanceSignal := &daisy.InstanceSignal{}
	instanceSignal.Name = vmname
	instanceSignal.Stopped = stopped

	// Waiting for stop and waiting for success match are mutually exclusive.
	if !stopped {
		instanceSignal.SerialOutput = serialOutput
	}

	waitForInstances := &daisy.WaitForInstancesSignal{instanceSignal}

	waitStep, err := t.wf.NewStep("wait-" + stepname)
	if err != nil {
		return nil, err
	}
	waitStep.WaitForInstancesSignal = waitForInstances

	return waitStep, nil
}

func (t *TestWorkflow) addStopStep(stepname, vmname string) (*daisy.Step, error) {
	stopInstances := &daisy.StopInstances{}
	stopInstances.Instances = append(stopInstances.Instances, vmname)

	stopInstancesStep, err := t.wf.NewStep("stop-" + stepname)
	if err != nil {
		return nil, err
	}
	stopInstancesStep.StopInstances = stopInstances

	return stopInstancesStep, nil
}

func (t *TestWorkflow) addDiskResizeStep(stepname, vmname string, diskSize int) (*daisy.Step, error) {
	resizeDisk := &daisy.ResizeDisk{}
	resizeDisk.DisksResizeRequest.SizeGb = int64(diskSize)
	resizeDisk.Name = vmname
	resizeDiskStepName := "resize-disk-" + stepname
	resizeDiskStep, err := t.wf.NewStep(resizeDiskStepName)
	if err != nil {
		return nil, err
	}
	resizeDiskStep.ResizeDisks = &daisy.ResizeDisks{resizeDisk}

	return resizeDiskStep, nil
}

func (t *TestWorkflow) addStartStep(stepname, vmname string) (*daisy.Step, error) {
	startInstances := &daisy.StartInstances{}
	startInstances.Instances = append(startInstances.Instances, vmname)

	startInstancesStep, err := t.wf.NewStep("start-" + stepname)
	if err != nil {
		return nil, err
	}
	startInstancesStep.StartInstances = startInstances

	return startInstancesStep, nil
}

func (t *TestWorkflow) appendCreateNetworkStep(networkName string, mtu int, autoCreateSubnetworks bool) (*daisy.Step, *daisy.Network, error) {
	network := &daisy.Network{
		Network: compute.Network{
			Name: networkName,
			Mtu:  int64(mtu),
		},
		AutoCreateSubnetworks: &autoCreateSubnetworks,
	}

	createNetworks := &daisy.CreateNetworks{}
	*createNetworks = append(*createNetworks, network)
	createNetworkStep, ok := t.wf.Steps[createNetworkStepName]
	if ok {
		// append to existing step.
		*createNetworkStep.CreateNetworks = append(*createNetworkStep.CreateNetworks, network)
	} else {
		var err error
		createNetworkStep, err = t.wf.NewStep(createNetworkStepName)
		if err != nil {
			return nil, nil, err
		}
		createNetworkStep.CreateNetworks = createNetworks
	}

	return createNetworkStep, network, nil
}

func (t *TestWorkflow) appendCreateSubnetworksStep(name, ipRange, networkName string) (*daisy.Step, *daisy.Subnetwork, error) {
	subnetwork := &daisy.Subnetwork{
		Subnetwork: compute.Subnetwork{
			Name:        name,
			IpCidrRange: ipRange,
			Network:     networkName,
		},
	}
	createSubnetworks := &daisy.CreateSubnetworks{}
	*createSubnetworks = append(*createSubnetworks, subnetwork)

	createSubnetworksStep, ok := t.wf.Steps[createSubnetworkStepName]
	if ok {
		// append to existing step.
		*createSubnetworksStep.CreateSubnetworks = append(*createSubnetworksStep.CreateSubnetworks, subnetwork)
	} else {
		var err error
		createSubnetworksStep, err = t.wf.NewStep(createSubnetworkStepName)
		if err != nil {
			return nil, nil, err
		}

		createSubnetworksStep.CreateSubnetworks = createSubnetworks
	}

	return createSubnetworksStep, subnetwork, nil
}

func getGCSPrefix(ctx context.Context, storageClient *storage.Client, project, gcsPath string) (string, error) {
	// Set global client.
	client = storageClient

	// If user didn't specify gcsPath, detect or create the bucket.
	if gcsPath == "" {
		bucket, err := daisyBucket(ctx, client, project)
		if err != nil {
			return "", fmt.Errorf("failed to find or create daisy bucket: %v", err)
		}
		gcsPath = fmt.Sprintf("gs://%s", bucket)
	}
	return fmt.Sprintf("%s/%s", strings.TrimSuffix(gcsPath, "/"), time.Now().Format(time.RFC3339)), nil
}

// finalizeWorkflows adds the final necessary data to each workflow for it to
// be able to run, including the final copy-objects step.
func finalizeWorkflows(ctx context.Context, tests []*TestWorkflow, zone, gcsPrefix, machineType string) error {
	log.Printf("Storing artifacts and logs in %s", gcsPrefix)
	for _, twf := range tests {
		if twf.wf == nil {
			return fmt.Errorf("found nil workflow in finalize")
		}

		twf.wf.StorageClient = client

		// $GCS_PATH/2021-04-20T11:44:08-07:00/image_validation/debian-10
		twf.gcsPath = fmt.Sprintf("%s/%s/%s", gcsPrefix, twf.Name, twf.ShortImage)
		twf.wf.GCSPath = twf.gcsPath

		twf.wf.Zone = zone

		arch := "amd64"
		if machineType != "" {
			createVMsStep := twf.wf.Steps[createVMsStepName]
			for _, vm := range createVMsStep.CreateInstances.Instances {
				vm.MachineType = machineType
			}
			if strings.HasPrefix(machineType, "t2a") {
				arch = "arm64"
			}
		}

		if strings.Contains(twf.Image, "windows") {
			archBits := "64"
			if strings.Contains(twf.Image, "x86") {
				archBits = "32"
			}
			twf.wf.Sources["testpackage"] = fmt.Sprintf("/%s%s.exe", twf.Name, archBits)
			twf.wf.Sources["wrapper.exe"] = fmt.Sprintf("%s%s.exe", testWrapperPathWindows, archBits)
		} else {
			twf.wf.Sources["testpackage"] = fmt.Sprintf("/%s.%s.test", twf.Name, arch)
			twf.wf.Sources["wrapper"] = fmt.Sprintf("%s.%s", testWrapperPath, arch)
		}

		// add a final copy-objects step which copies the daisy-outs-path directory to twf.gcsPath + /outs
		copyGCSObject := daisy.CopyGCSObject{}
		copyGCSObject.Source = "${OUTSPATH}/" // Trailing slash apparently crucial.
		copyGCSObject.Destination = twf.gcsPath + "/outs"
		copyGCSObjects := &daisy.CopyGCSObjects{copyGCSObject}
		copyStep, err := twf.wf.NewStep("copy-objects")
		if err != nil {
			return fmt.Errorf("failed to add copy-objects step to workflow %s: %v", twf.Name, err)
		}
		copyStep.CopyGCSObjects = copyGCSObjects

		// The "copy-objects" step depends on every wait step.
		for stepname, step := range twf.wf.Steps {
			if !strings.HasPrefix(stepname, "wait-") {
				continue
			}
			if err := twf.wf.AddDependency(copyStep, step); err != nil {
				return fmt.Errorf("failed to add copy-objects step: %v", err)
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
		out, err := utils.DownloadGCSObject(ctx, client, vm.Metadata["_test_results_url"])
		if err != nil {
			return nil, fmt.Errorf("failed to get results for test %s vm %s: %v", ts.Name, vm.Name, err)
		}
		results = append(results, string(out))
	}

	return results, nil
}

// NewTestWorkflow returns a new TestWorkflow.
func NewTestWorkflow(name, image, timeout string) (*TestWorkflow, error) {
	t := &TestWorkflow{}
	t.counter = 0
	t.Name = name
	t.Image = image

	parts := strings.Split(image, "/")
	t.ShortImage = parts[len(parts)-1]

	t.wf = daisy.New()
	t.wf.Name = strings.ReplaceAll(name, "_", "-")
	t.wf.DefaultTimeout = timeout

	t.wf.DisableGCSLogging()
	t.wf.DisableCloudLogging()
	t.wf.DisableStdoutLogging()

	return t, nil
}

// PrintTests prints all test workflows.
func PrintTests(ctx context.Context, storageClient *storage.Client, testWorkflows []*TestWorkflow, project, zone, gcsPath, machineType string) {
	gcsPrefix, err := getGCSPrefix(ctx, storageClient, project, gcsPath)
	if err != nil {
		log.Printf("Error determining GCS prefix: %v", err)
		gcsPrefix = ""
	}
	if err := finalizeWorkflows(ctx, testWorkflows, zone, gcsPrefix, machineType); err != nil {
		log.Printf("Error finalizing workflow: %v", err)
	}
	for _, test := range testWorkflows {
		if test.wf == nil {
			log.Printf("%s test on image %s: workflow was nil, skipping", test.Name, test.ShortImage)
			continue
		}
		test.wf.Print(ctx)
	}
}

// ValidateTests validates all test workflows.
func ValidateTests(ctx context.Context, storageClient *storage.Client, testWorkflows []*TestWorkflow, project, zone, gcsPath, machineType string) error {
	gcsPrefix, err := getGCSPrefix(ctx, storageClient, project, gcsPath)
	if err != nil {
		return err
	}
	if err := finalizeWorkflows(ctx, testWorkflows, zone, gcsPrefix, machineType); err != nil {
		return err
	}
	for _, test := range testWorkflows {
		log.Printf("Validating test %s on image %s\n", test.Name, test.ShortImage)
		if test.wf == nil {
			return fmt.Errorf("%s test on image %s: workflow was nil", test.Name, test.ShortImage)
		}
		if err := test.wf.Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// daisyBucket returns the bucket name for outputs, creating it if needed.
func daisyBucket(ctx context.Context, client *storage.Client, project string) (string, error) {
	bucketName := strings.Replace(project, ":", "-", -1) + "-cloud-test-outputs"
	it := client.Buckets(ctx, project)
	for attr, err := it.Next(); err != iterator.Done; attr, err = it.Next() {
		if err != nil {
			return "", fmt.Errorf("failed to iterate buckets: %v", err)
		}
		if attr.Name == bucketName {
			return bucketName, nil
		}
	}

	if err := client.Bucket(bucketName).Create(ctx, project, nil); err != nil {
		return "", fmt.Errorf("failed to create bucket: %v", err)
	}
	return bucketName, nil
}

// RunTests runs all test workflows.
func RunTests(ctx context.Context, storageClient *storage.Client, testWorkflows []*TestWorkflow, project, zone, gcsPath, machineType string, parallelCount int, testProjects []string) (*TestSuites, error) {
	gcsPrefix, err := getGCSPrefix(ctx, storageClient, project, gcsPath)
	if err != nil {
		return nil, err
	}

	finalizeWorkflows(ctx, testWorkflows, zone, gcsPrefix, machineType)

	testResults := make(chan testResult, len(testWorkflows))
	testchan := make(chan *TestWorkflow, len(testWorkflows))

	exclusiveProjects := make(chan string, len(testProjects))
	for _, proj := range testProjects {
		// Prefill with projects.
		exclusiveProjects <- proj
	}

	projects := make(chan string, len(testWorkflows))
	for idx := 0; idx < len(testWorkflows); idx++ {
		// Stripe all test projects
		projects <- testProjects[idx%len(testProjects)]
	}
	close(projects)

	var wg sync.WaitGroup
	for i := 0; i < parallelCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for test := range testchan {
				if test.lockProject {
					// This will block until an exclusive project is available.
					log.Printf("test %s/%s requires write lock for project", test.Name, test.ShortImage)
					test.wf.Project = <-exclusiveProjects
				} else {
					test.wf.Project = <-projects
				}
				testResults <- runTestWorkflow(ctx, test)
				if test.lockProject {
					// "unlock" the project.
					exclusiveProjects <- test.wf.Project
				}
			}
		}(i)
	}
	for _, ts := range testWorkflows {
		testchan <- ts
	}
	close(testchan)
	wg.Wait()

	var suites TestSuites
	for i := 0; i < len(testWorkflows); i++ {
		suites.TestSuite = append(suites.TestSuite, parseResult(<-testResults))
	}
	for _, suite := range suites.TestSuite {
		suites.Errors += suite.Errors
		suites.Failures += suite.Failures
		suites.Tests += suite.Tests
		suites.Disabled += suite.Disabled
		suites.Skipped += suite.Skipped
		suites.Time += suite.Time
	}

	return &suites, nil
}

func formatTimeDelta(format string, t time.Duration) string {
	z := time.Unix(0, 0).UTC()
	return z.Add(time.Duration(t)).Format(format)
}

func runTestWorkflow(ctx context.Context, test *TestWorkflow) testResult {
	var res testResult
	res.testWorkflow = test
	if test.skipped {
		res.skipped = true
		res.err = fmt.Errorf("test suite was skipped with message: %q", res.testWorkflow.SkippedMessage())
		return res
	}

	start := time.Now()
	log.Printf("running test %s/%s (ID %s) in project %s\n", test.Name, test.ShortImage, test.wf.ID(), test.wf.Project)
	if err := test.wf.Run(ctx); err != nil {
		res.err = err
		return res
	}

	delta := formatTimeDelta("04m 05s", time.Now().Sub(start))
	log.Printf("finished test %s/%s (ID %s) in project %s, time spent: %s\n", test.Name, test.ShortImage, test.wf.ID(), test.wf.Project, delta)
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
	name := fmt.Sprintf("%s-%s", res.testWorkflow.Name, res.testWorkflow.ShortImage)

	switch {
	case res.skipped:
		for _, test := range getTestsBySuiteName(res.testWorkflow.Name) {
			tc := &testCase{}
			tc.Classname = name
			tc.Name = test
			tc.Skipped = &junitSkipped{res.testWorkflow.SkippedMessage()}
			ret.TestCase = append(ret.TestCase, tc)

			ret.Tests++
			ret.Skipped++
		}
	case res.workflowSuccess:
		// Workflow completed without error. Only in this case do we try to parse the result.
		ret = convertToTestSuite(res.results, name)
	default:
		var status string
		if res.err != nil {
			status = res.err.Error()
		} else {
			status = "Unknown status"
		}
		for _, test := range getTestsBySuiteName(res.testWorkflow.Name) {
			tc := &testCase{}
			tc.Classname = name
			tc.Name = test
			tc.Failure = &junitFailure{status, "Failure"}
			ret.TestCase = append(ret.TestCase, tc)

			ret.Tests++
			ret.Failures++
		}
	}

	ret.Name = name
	return ret
}

func getTestsBySuiteName(name string) []string {
	b, err := ioutil.ReadFile(fmt.Sprintf("/%s_tests.txt", name))
	if err != nil {
		log.Fatalf("unable to parse tests list: %v", err)
		return []string{} // NOT nil
	}
	var res []string
	for _, testname := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(testname, "Test") {
			res = append(res, testname)
		}
	}
	return res
}

func (t *TestWorkflow) getLastStepForVM(vmname string) (*daisy.Step, error) {
	step := "wait-" + vmname
	if _, ok := t.wf.Steps[step]; !ok {
		return nil, fmt.Errorf("no step " + step)
	}
	rdeps := make(map[string][]string)
	for dependent, dependencies := range t.wf.Dependencies {
		for _, dependency := range dependencies {
			rdeps[dependency] = append(rdeps[dependency], dependent)
		}
	}

	for {
		deps, ok := rdeps[step]
		if !ok {
			// no more steps depend on this one
			break
		}
		if len(deps) > 1 {
			return nil, fmt.Errorf("workflow has non-linear dependencies")
		}
		step = deps[0]
	}
	return t.wf.Steps[step], nil
}
