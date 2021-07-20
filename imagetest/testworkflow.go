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
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
	"github.com/google/uuid"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iterator"
)

var (
	client *storage.Client
)

const (
	testWrapperPath = "/wrapper"
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
}

// Network represent network used by vm in setup.go.
type Network struct {
	name         string
	testWorkflow *TestWorkflow
}

// SubNetwork represent subnetwork used by vm in setup.go.
type SubNetwork struct {
	name         string
	testWorkflow *TestWorkflow
}

// AddSecondaryRange add secondary IP range to SubNetwork
func (s SubNetwork) AddSecondaryRange(rangeName, ipRange string) {
	for _, subnetwork := range *s.testWorkflow.wf.Steps[createSubNetworkStepName].CreateSubnetworks {
		if subnetwork.Name == s.name {
			subnetwork.SecondaryIpRanges = append(subnetwork.SecondaryIpRanges, &compute.SubnetworkSecondaryRange{
				IpCidrRange: ipRange,
				RangeName:   rangeName,
			})
		}
	}
}

// SingleVMTest configures one VM running tests.
func SingleVMTest(t *TestWorkflow) error {
	_, err := t.CreateTestVM("vm")
	return err
}

// Skip marks a test workflow to be skipped.
func (t *TestWorkflow) Skip(message string) {
	t.skipped = true
	t.skippedMessage = message
}

// SkippedMessage returns the skip reason message for the workflow.
func (t *TestWorkflow) SkippedMessage() string {
	return t.skippedMessage
}

// CreateTestVM creates the necessary steps to create a VM with the specified name to the workflow.
func (t *TestWorkflow) CreateTestVM(name string) (*TestVM, error) {
	parts := strings.Split(name, ".")
	vmname := strings.ReplaceAll(parts[0], "_", "-")

	createDisksStep, err := t.appendCreateDisksStep(vmname)
	if err != nil {
		return nil, err
	}

	// createDisksStep doesn't depend on any other steps.
	createVMStep, err := t.appendCreateVMStep(vmname, name)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(createVMStep, createDisksStep); err != nil {
		return nil, err
	}

	waitStep, err := t.addWaitStep(vmname, vmname, false)
	if err != nil {
		return nil, err
	}

	if err := t.wf.AddDependency(waitStep, createVMStep); err != nil {
		return nil, err
	}

	return &TestVM{name: vmname, testWorkflow: t}, nil
}

func (t *TestWorkflow) appendCreateVMStep(name, hostname string) (*daisy.Step, error) {
	attachedDisk := &compute.AttachedDisk{Source: name}

	instance := &daisy.Instance{}
	instance.StartupScript = "wrapper"
	instance.Name = name
	instance.Scopes = append(instance.Scopes, "https://www.googleapis.com/auth/devstorage.read_write")
	instance.Disks = append(instance.Disks, attachedDisk)
	if hostname != "" && name != hostname {
		instance.Hostname = hostname
	}

	instance.Metadata = make(map[string]string)
	instance.Metadata["_test_vmname"] = name
	instance.Metadata["_test_package_url"] = "${SOURCESPATH}/testpackage"
	instance.Metadata["_test_results_url"] = fmt.Sprintf("${OUTSPATH}/%s.txt", name)

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
			return nil, err
		}
		createVMStep.CreateInstances = createInstances
	}

	return createVMStep, nil
}

func (t *TestWorkflow) appendCreateDisksStep(diskname string) (*daisy.Step, error) {
	bootdisk := &daisy.Disk{}
	bootdisk.Name = diskname
	bootdisk.SourceImage = t.Image

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

func (t *TestWorkflow) appendCreateNetworkStep(networkName string, autoCreateSubnetworks bool) (*daisy.Step, error) {
	network := &daisy.Network{
		Network: compute.Network{
			Name: networkName,
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
			return nil, err
		}
		createNetworkStep.CreateNetworks = createNetworks
	}

	return createNetworkStep, nil
}

func (t *TestWorkflow) appendCreateSubnetworksStep(name, ipRange, networkName string) (*daisy.Step, error) {
	subnetwork := &daisy.Subnetwork{
		Subnetwork: compute.Subnetwork{
			Name:        name,
			IpCidrRange: ipRange,
			Network:     networkName,
		},
	}
	createSubnetworks := &daisy.CreateSubnetworks{}
	*createSubnetworks = append(*createSubnetworks, subnetwork)

	createSubnetworksStep, ok := t.wf.Steps[createSubNetworkStepName]
	if ok {
		// append to existing step.
		*createSubnetworksStep.CreateSubnetworks = append(*createSubnetworksStep.CreateSubnetworks, subnetwork)
	} else {
		var err error
		createSubnetworksStep, err = t.wf.NewStep(createSubNetworkStepName)
		if err != nil {
			return nil, err
		}

		createSubnetworksStep.CreateSubnetworks = createSubnetworks
	}

	return createSubnetworksStep, nil
}

// CreateSubNetwork creates custom subnetwork. Using SetCustomNetwork method
// provided by TestVM to config network on vm
func (n Network) CreateSubNetwork(name string, ipRange string) (*SubNetwork, error) {
	createSubnetworksStep, err := n.testWorkflow.appendCreateSubnetworksStep(name, ipRange, n.name)
	if err != nil {
		return nil, err
	}
	firstStep, ok := n.testWorkflow.wf.Steps[createVMsStepName]
	if !ok {
		return nil, fmt.Errorf("failed resolve first step")
	}
	createNetworkStep, ok := n.testWorkflow.wf.Steps[createNetworkStepName]
	if !ok {
		return nil, fmt.Errorf("create-network step missing")
	}
	if err := n.testWorkflow.wf.AddDependency(createSubnetworksStep, createNetworkStep); err != nil {
		return nil, err
	}
	if err := n.testWorkflow.wf.AddDependency(firstStep, createSubnetworksStep); err != nil {
		return nil, err
	}

	return &SubNetwork{name, n.testWorkflow}, nil
}

// CreateNetwork creates custom network. Using SetCustomNetwork method provided by
// TestVM to config network on vm
func (t *TestWorkflow) CreateNetwork(networkName string, autoCreateSubnetworks bool) (*Network, error) {
	createNetworkStep, err := t.appendCreateNetworkStep(networkName, autoCreateSubnetworks)
	if err != nil {
		return nil, err
	}

	firstStep, ok := t.wf.Steps[createVMsStepName]
	if !ok {
		return nil, fmt.Errorf("failed resolve first step")
	}
	if err := t.wf.AddDependency(firstStep, createNetworkStep); err != nil {
		return nil, err
	}

	return &Network{networkName, t}, nil
}

// finalizeWorkflows adds the final necessary data to each workflow for it to
// be able to run, including the final copy-objects step.
func finalizeWorkflows(tests []*TestWorkflow, zone, project, bucket string) error {
	run := time.Now().Format(time.RFC3339)
	for _, ts := range tests {
		if ts.wf == nil {
			return fmt.Errorf("found nil workflow in finalize")
		}

		// gs://$PROJECT-cloud-test-outputs/2021-04-20T11:44:08-07:00/image_validation/debian-10
		ts.gcsPath = fmt.Sprintf("gs://%s/%s/%s/%s", bucket, run, ts.Name, ts.ShortImage)
		ts.wf.GCSPath = ts.gcsPath

		ts.wf.DisableGCSLogging()
		ts.wf.DisableCloudLogging()
		ts.wf.DisableStdoutLogging()

		ts.wf.Zone = zone
		ts.wf.Project = project

		ts.wf.Sources["wrapper"] = testWrapperPath
		ts.wf.Sources["testpackage"] = fmt.Sprintf("/%s.test", ts.Name)

		// add a final copy-objects step which copies the daisy-outs-path directory to ts.gcsPath + /outs
		copyGCSObject := daisy.CopyGCSObject{}
		copyGCSObject.Source = "${OUTSPATH}/" // Trailing slash apparently crucial.
		copyGCSObject.Destination = ts.gcsPath + "/outs"
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

	return t, nil
}

// PrintTests prints all test workflows.
func PrintTests(ctx context.Context, testWorkflows []*TestWorkflow, project, zone string) {
	finalizeWorkflows(testWorkflows, zone, project, "")
	for _, test := range testWorkflows {
		if test.wf == nil {
			continue
		}
		test.wf.Print(ctx)
	}
}

// ValidateTests validates all test workflows.
func ValidateTests(ctx context.Context, testWorkflows []*TestWorkflow, project, zone string) error {
	finalizeWorkflows(testWorkflows, zone, project, "")
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
func RunTests(ctx context.Context, testWorkflows []*TestWorkflow, project, zone string, parallelCount int) (*TestSuites, error) {
	var err error
	client, err = storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to set up storage client: %v", err)
	}
	bucket, err := daisyBucket(ctx, client, project)
	if err != nil {
		return nil, err
	}
	finalizeWorkflows(testWorkflows, zone, project, bucket)

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

func runTestWorkflow(ctx context.Context, test *TestWorkflow) testResult {
	var res testResult
	res.testWorkflow = test
	if test.skipped {
		res.skipped = true
		res.err = fmt.Errorf("Test suite was skipped with message: %q", res.testWorkflow.SkippedMessage())
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

// AddSSHKey generate ssh key pair and return public key.
func (t *TestWorkflow) AddSSHKey(user string) (string, error) {
	keyFileName := "/id_rsa_" + uuid.New().String()
	if _, err := os.Stat(keyFileName); os.IsExist(err) {
		os.Remove(keyFileName)
	}
	commandArgs := []string{"-t", "rsa", "-f", keyFileName, "-N", "", "-q"}
	cmd := exec.Command("ssh-keygen", commandArgs...)
	if err := cmd.Run(); err != nil {
		return "", err
	}

	publicKey, err := ioutil.ReadFile(keyFileName + ".pub")
	if err != nil {
		return "", err
	}
	sourcePath := fmt.Sprintf("%s-ssh-key", user)
	t.wf.Sources[sourcePath] = keyFileName

	return string(publicKey), nil
}
