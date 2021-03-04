package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"

	"github.com/GoogleCloudPlatform/compute-image-tools/daisy"
	"guest-test-infra/test_manager/test_manager"
	"guest-test-infra/test_manager/test_suites/shutdown_scripts"
	"guest-test-infra/test_manager/test_suites/image_validation"
	"guest-test-infra/test_manager/test_suites/oslogin"
)

const defaultImage = "debian.rhel,ubuntu,windows"

var (
	workProject = flag.String("work_project", "", "project to perform the work in, passed to Daisy as workflow project, will override WorkProject in template")
	gcsPath     = flag.String("gcs_path", "", "GCS bucket to use, overrides what is set in workflow")

	image = flag.String("image", "", "a list of image we want to run E2E test against")
)

var testStepFunctions = []func(){
	shutdown_scripts.Setup,
	image_validation.Setup,
	oslogin.Setup,
}

func main() {
	flag.Parse()
	images := strings.Split(*image, ",")

	// call Setup function to create E2E test
	for _, testSetupFunc := range testStepFunctions {
		testSetupFunc()
	}
	test_manager.E2eTestManagerInstance.WorkProject = *workProject
	test_manager.E2eTestManagerInstance.GcsPath = *gcsPath

	ctx := context.Background()
	ctx, errs, ws := createWorkflow(ctx, images)

	var wg sync.WaitGroup
	errors := make(chan error, len(ws)+len(errs))
	for _, w := range ws {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		go func(w *daisy.Workflow) {
			select {
			case <-c:
				fmt.Printf("\nCtrl-C caught, sending cancel signal to %q...\n", w.Name)
				close(w.Cancel)
				errors <- fmt.Errorf("workflow %q was canceled", w.Name)
			case <-w.Cancel:
			}
		}(w)

		wg.Add(1)
		go func(w *daisy.Workflow) {
			defer wg.Done()
			fmt.Printf("\n[E2E Test] Running workflow %q\n", w.Name)
			if err := w.Run(ctx); err != nil {
				errors <- fmt.Errorf("%s: %v", w.Name, err)
				return
			}
			fmt.Printf("\n[E2E Test] Workflow %q finished\n", w.Name)
		}(w)
	}
	wg.Wait()

	checkError(errors)
}

func createWorkflow(ctx context.Context, images []string) (context.Context, []error, []*daisy.Workflow) {
	var ws []*daisy.Workflow
	var errs []error

	for _, image := range images {
		w, err := test_manager.E2eTestManagerInstance.CreateWorkflow(image)
		if w != nil {
		}
		if err != nil {
			err = fmt.Errorf("workflow creation error: %s", err)
			errs = append(errs, err)
			continue
		}
		if err := w.PopulateClients(ctx); err != nil {
			errs = append(errs, err)
			continue
		}
		if w != nil {
			ws = append(ws, w)
		}
	}
	return ctx, errs, ws
}

func checkError(errors chan error) {
	select {
	case err := <-errors:
		fmt.Fprintln(os.Stderr, "\n[E2E Test] Errors in one or more workflows:")
		fmt.Fprintln(os.Stderr, " ", err)
		for {
			select {
			case err := <-errors:
				fmt.Fprintln(os.Stderr, " ", err)
				continue
			default:
				os.Exit(1)
			}
		}
	default:
		return
	}
}
