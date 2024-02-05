package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

// In special cases such as the shutdown script, the guest attribute match
// on the first boot must have a different name than the usual guest attribute.
func checkFirstBootSpecialGA(ctx context.Context) bool {
	if _, err := utils.GetMetadata(ctx, "instance", "attributes", "shouldRebootDuringTest"); err == nil {
		_, foundFirstBootGA := utils.GetMetadata(ctx, "instance", "guest-attributes",
			utils.GuestAttributeTestNamespace, utils.FirstBootGAKey)
		// if the special attribute to match the first boot of the shutdown script test is already set, foundFirstBootGA will be nil and we should use the regular guest attribute.
		if foundFirstBootGA != nil {
			return true
		}
	}
	return false
}

func main() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create cloud storage client: %v", err)
	}
	log.Printf("FINISHED-BOOTING")
	firstBootSpecialAttribute := checkFirstBootSpecialGA(ctx)
	// firstBootSpecialGA should be true if we need to match a different guest attribute than the usual guest attribute
	defer func(ctx context.Context, firstBootSpecialGA bool) {
		var err error
		if firstBootSpecialGA {
			err = utils.PutMetadata(ctx, path.Join("instance", "guest-attributes", utils.GuestAttributeTestNamespace,
				utils.FirstBootGAKey), "")
		} else {
			err = utils.PutMetadata(ctx, path.Join("instance", "guest-attributes", utils.GuestAttributeTestNamespace,
				utils.GuestAttributeTestKey), "")
		}

		if err != nil {
			log.Printf("could not place guest attribute key to end test")
		}
		for f := 0; f < 5; f++ {
			log.Printf("FINISHED-TEST")
			time.Sleep(1 * time.Second)
		}
	}(ctx, firstBootSpecialAttribute)

	daisyOutsPath, err := utils.GetMetadata(ctx, "instance", "attributes", "daisy-outs-path")
	if err != nil {
		log.Fatalf("failed to get metadata daisy-outs-path: %v", err)
	}
	daisyOutsPath = daisyOutsPath + "/"

	testPackageURL, err := utils.GetMetadata(ctx, "instance", "attributes", "_test_package_url")
	if err != nil {
		log.Fatalf("failed to get metadata _test_package_url: %v", err)
	}

	resultsURL, err := utils.GetMetadata(ctx, "instance", "attributes", "_test_results_url")
	if err != nil {
		log.Fatalf("failed to get metadata _test_results_url: %v", err)
	}

	var testArguments = []string{"-test.v"}

	testRun, err := utils.GetMetadata(ctx, "instance", "attributes", "_test_run")
	if err == nil && testRun != "" {
		testArguments = append(testArguments, "-test.run", testRun)
	}

	testPackage, err := utils.GetMetadata(ctx, "instance", "attributes", "_test_package_name")
	if err != nil {
		log.Fatalf("failed to get metadata _test_package_name: %v", err)
	}

	// NOTE(sejalsharma): modified the following line "", "image_test".
	workDir, err := ioutil.TempDir("/etc", "iimage_test")
	if err != nil {
		log.Fatalf("failed to create work dir: %v", err)
	}
	workDir = workDir + "/"

	if err = utils.DownloadGCSObjectToFile(ctx, client, testPackageURL, workDir+testPackage); err != nil {
		log.Fatalf("failed to download object: %v", err)
	}

	log.Printf("sleep 30s to allow environment to stabilize")
	time.Sleep(30 * time.Second)

	out, err := executeCmd(workDir+testPackage, workDir, testArguments)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			log.Printf("test package exited with error: %v stderr: %q", ee, ee.Stderr)
		} else {
			log.Fatalf("failed to execute test package: %v stdout: %q", err, out)
		}
	}

	log.Printf("command output:\n%s\n", out)

	if err = uploadGCSObject(ctx, client, resultsURL, bytes.NewReader(out)); err != nil {
		log.Fatalf("failed to upload test result: %v", err)
	}
}

func executeCmd(cmd, dir string, arg []string) ([]byte, error) {
	command := exec.Command(cmd, arg...)
	command.Dir = dir
	log.Printf("Going to execute: %q", command.String())

	output, err := command.Output()
	if err != nil {
		return output, err
	}
	return output, nil
}

func uploadGCSObject(ctx context.Context, client *storage.Client, path string, data io.Reader) error {
	u, err := url.Parse(path)
	if err != nil {
		log.Fatalf("failed to parse gcs url: %v", err)
	}
	object := strings.TrimPrefix(u.Path, "/")
	log.Printf("uploading to bucket %s object %s\n", u.Host, object)

	dst := client.Bucket(u.Host).Object(object).NewWriter(ctx)
	if _, err := io.Copy(dst, data); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	dst.Close()
	return nil
}
