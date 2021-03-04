package test_manager

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"google.golang.org/api/compute/v1"
)

const workZone = "us-west1-a"
const (
	debian_10 = "projects/debian-cloud/global/images/family/debian-10"
	rhel_7    = "projects/rhel-cloud/global/images/family/rhel-7"
	rhel_8    = "projects/rhel-cloud/global/images/family/rhel-8"
)

var E2eTestManagerInstance *E2ETestManager
var once sync.Once

// An E2ETestManager represent a set of E2E tests run by daisy
type E2ETestManager struct {
	WorkProject string
	GcsPath     string
	Metadata    map[string]string

	ImageTests []*ImageTest
}

// An ImageTest represent an subset of E2E tests that could be invoke for a list of image
type ImageTest struct {
	ImageName []string
	// Pass to instance metadata
	testFunc       []string
	ShutdownScript string
}

type testManager interface {
	Config()
	AddShutdownScript()
	CreateImageTest() *ImageTest
}

func (iT *ImageTest) RunAllTests() {
	iT.testFunc = []string{}
}
func (iT *ImageTest) RunTests(test ...string) {
	for _, t := range test {
		iT.testFunc = append(iT.testFunc, t)
	}
}

// Remove default image
func (iT *ImageTest) AddSkipImages(images ...string) {
	for _, image := range images {
		for _, imageName := range iT.ImageName {
			if imageName == image {
				iT.ImageName = remove(iT.ImageName, image)
				break
			}
		}
	}
}

func (iT *ImageTest) AddShutdownScript(script string) {
	iT.ShutdownScript = script
}

func remove(l []string, item string) []string {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}

func New() *E2ETestManager {
	// Singleton
	once.Do(func() {
		E2eTestManagerInstance = &E2ETestManager{
			WorkProject: "gcp-guest",
			GcsPath:     "",
			ImageTests:  []*ImageTest{},
			Metadata:    map[string]string{},
		}
	})
	return E2eTestManagerInstance
}

func (t *E2ETestManager) CreateImageTest() *ImageTest {
	return &ImageTest{
		// default run on all images
		ImageName: []string{debian_10, rhel_7, rhel_8},
		// default run all test cases
		testFunc:       []string{},
		ShutdownScript: nil,
	}
}

// Create a workflow for a image distros and run all tests or a subset of test available for this image
func (t *E2ETestManager) CreateWorkflow(image string) (*daisy.Workflow, error) {
	var w *daisy.Workflow
	var testFunc []string
	for _, iTests := range t.ImageTests {
		if contains(iTests.ImageName, image) {
			testFunc = append(testFunc, iTests.testFunc...)
		}
	}

	fmt.Printf("Creating E2E Testing Workflows\n")
	w, err := createWorkflow(t.WorkProject, image, testFunc, t.Metadata)
	if err != nil {
		return nil, err
	}

	if len(w.Steps) == 0 {
		return nil, nil
	}
	if t.GcsPath != "" {
		w.GCSPath = t.GcsPath
	}
	if w == nil {
		err = fmt.Errorf("")
		return nil, nil
	}
	return w, nil
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (t *E2ETestManager) AddImageTest(imageTest ...*ImageTest) {
	for _, iT := range imageTest {
		t.ImageTests = append(t.ImageTests, iT)
	}
}

func (t *E2ETestManager) AddMetadata(key string, value string) {
	t.Metadata[key] = value
}

// create a workflow for an image
func createWorkflow(workProject, image string, testFunc []string, metadata map[string]string) (*daisy.Workflow, error) {
	w := daisy.New()
	w.Name = "e2e-test-" + randString(5)
	w.Project = workProject
	w.Sources["startup_script"] = "bootstrap.sh"
	w.Sources["test_wrapper.sh"] = "test_wrapper.sh"

	var err error
	fmt.Printf("Create Image Test for Image %s\n", image)
	w, err = createWorkflowStep(w, workProject, image, testFunc, metadata)

	if err != nil {
		return nil, err
	}
	return w, nil
}

func createWorkflowStep(w *daisy.Workflow, image, workProject string, testFunc []string, metadata map[string]string) (*daisy.Workflow, error) {
	var diskName = "disk"
	var instanceName = "instance"

	cds := createDisks(image, workProject, diskName)
	cins := createInstances(workProject, diskName, instanceName, testFunc, metadata)
	ws := waitForInstancesSignal(instanceName)
	drs := deleteResource(diskName, image, instanceName)

	w, err := populateSteps(w, nil, cds, cins, ws, drs)
	if err != nil {
		return nil, err
	}
	return w, nil
}

//func createImages(imageName, workProject, tarBallPath string) *daisy.CreateImages {
//	ci := daisy.Image{
//		Image: compute.Image{
//			Name:    imageName,
//			RawDisk: &compute.ImageRawDisk{Source: tarBallPath},
//		},
//		ImageBase: daisy.ImageBase{
//			Resource: daisy.Resource{
//				NoCleanup: true,
//				Project:   workProject,
//				RealName:  imageName,
//			},
//			IgnoreLicenseValidationIfForbidden: true,
//		},
//	}
//	cis := &daisy.CreateImages{Images: []*daisy.Image{&ci}}
//	return cis
//}

func createDisks(imageName string, workProject, diskName string) *daisy.CreateDisks {
	cd := daisy.Disk{
		Disk: compute.Disk{
			Name:        diskName,
			Zone:        workZone,
			SourceImage: imageName,
		},
		Resource: daisy.Resource{
			Project: workProject,
		},
		IsWindows:            "false",
		SizeGb:               "20",
		FallbackToPdStandard: false,
	}
	cds := daisy.CreateDisks{&cd}
	return &cds
}

func createInstances(workProject, diskName, instanceName string, testFunc []string, metadata map[string]string) *daisy.CreateInstances {
	cin := daisy.Instance{
		Instance: compute.Instance{
			Name: instanceName,
			Zone: workZone,
			Disks: []*compute.AttachedDisk{
				{Source: diskName},
			},
		},
		InstanceBase: daisy.InstanceBase{
			Resource: daisy.Resource{
				Project: workProject,
			},
			// Bootstrap script to download test wrapper
			StartupScript: "startup_script",
		},
		Metadata: map[string]string{
			// Test binary used in test wrapper to invoke
			"test_binary_path": "",
			// TODO test.run only support regex
			"test.run_argument": strings.Join(testFunc, ","),
			"files_gcs_dir":     "${SOURCESPATH}",
		},
	}
	cins := &daisy.CreateInstances{Instances: []*daisy.Instance{&cin}}
	return cins
}

func waitForInstancesSignal(instanceName string) *daisy.WaitForInstancesSignal {
	wfis := daisy.InstanceSignal{
		Name: instanceName,
		SerialOutput: &daisy.SerialOutput{
			Port:         1,
			SuccessMatch: "E2ESuccess",
			FailureMatch: []string{"E2EFailed"},
			StatusMatch:  "E2EStatus",
		},
	}
	wfiss := &daisy.WaitForInstancesSignal{&wfis}
	return wfiss
}

func deleteResource(diskName, imageName, instanceName string) *daisy.DeleteResources {
	return &daisy.DeleteResources{
		Disks:     []string{diskName},
		Images:    []string{imageName},
		Instances: []string{instanceName},
	}
}

func populateSteps(w *daisy.Workflow, cis *daisy.CreateImages, cds *daisy.CreateDisks, cins *daisy.CreateInstances, ws *daisy.WaitForInstancesSignal, drs *daisy.DeleteResources) (*daisy.Workflow, error) {
	var err error

	var createImageStep *daisy.Step
	var createDiskStep *daisy.Step
	var createInstanceStep *daisy.Step
	var waitStep *daisy.Step
	var deleteStep *daisy.Step

	if cis != nil {
		createImageStep, err = w.NewStep("create-image")
		if err != nil {
			return nil, err
		}
		createImageStep.CreateImages = cis
	}

	if cds != nil {
		createDiskStep, err = w.NewStep("create-disk")
		if err != nil {
			return nil, err
		}
		createDiskStep.CreateDisks = cds
	}

	if cins != nil {
		createInstanceStep, err = w.NewStep("create-instance")
		if err != nil {
			return nil, err
		}
		createInstanceStep.CreateInstances = cins
	}

	if ws != nil {
		waitStep, err = w.NewStep("wait")
		if err != nil {
			return nil, err
		}
		waitStep.WaitForInstancesSignal = ws
	}

	if drs != nil {
		deleteStep, err = w.NewStep("delete")
		if err != nil {
			return nil, err
		}
		deleteStep.DeleteResources = drs
	}

	if createDiskStep != nil && createImageStep != nil {
		w.AddDependency(createDiskStep, createImageStep)
	}
	if createInstanceStep != nil && createDiskStep != nil {
		w.AddDependency(createInstanceStep, createDiskStep)
	}
	if waitStep != nil && createInstanceStep != nil {
		w.AddDependency(waitStep, createInstanceStep)
	}
	if deleteStep != nil && waitStep != nil {
		w.AddDependency(deleteStep, waitStep)
	}
	return w, nil
}

func randString(n int) string {
	gen := rand.New(rand.NewSource(time.Now().UnixNano()))
	letters := "bdghjlmnpqrstvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[gen.Int63()%int64(len(letters))]
	}
	return string(b)
}
