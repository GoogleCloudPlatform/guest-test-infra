package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func main() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create cloud storage client: %v", err)
	}
	log.Printf("FINISHED-BOOTING")
	defer func() {
		firstBootIgnoreTest := false
		if shouldRebootDuringTest, err := utils.GetMetadataAttribute("shouldRebootDuringTest"); err == nil {
			firstbootval, foundKey := utils.GetMetadataGuestAttribute(utils.GuestAttributeTestNamespace + "/" +  utils.FirstBootGAKey)
			// if first boot and the attribute is not found
			if foundKey != nil {
				firstBootIgnoreTest = true
			}
			log.Printf("found should boot variables %s %s and foundkey %v and boot bool %t", shouldRebootDuringTest, firstbootval, foundKey, firstBootIgnoreTest)
		} else {
			log.Printf("did not find the metadata")
		}
		var err error
		if firstBootIgnoreTest {
			err = utils.QueryMetadataGuestAttribute(ctx, utils.GuestAttributeTestNamespace, utils.FirstBootGAKey, http.MethodPut)
		} else {
			err = utils.QueryMetadataGuestAttribute(ctx, utils.GuestAttributeTestNamespace, utils.GuestAttributeTestKey, http.MethodPut)
		}

		if err != nil {
			log.Printf("could not place guest attribute key to end test")
		}
		for f := 0; f < 5; f++ {
			log.Printf("FINISHED-TEST")
			time.Sleep(1 * time.Second)
		}
	}()

	daisyOutsPath, err := utils.GetMetadataAttribute("daisy-outs-path")
	if err != nil {
		log.Fatalf("failed to get metadata daisy-outs-path: %v", err)
	}
	daisyOutsPath = daisyOutsPath + "/"

	testPackageURL, err := utils.GetMetadataAttribute("_test_package_url")
	if err != nil {
		log.Fatalf("failed to get metadata _test_package_url: %v", err)
	}

	resultsURL, err := utils.GetMetadataAttribute("_test_results_url")
	if err != nil {
		log.Fatalf("failed to get metadata _test_results_url: %v", err)
	}

	var testArguments = []string{"-test.v"}

	testRun, err := utils.GetMetadataAttribute("_test_run")
	if err == nil && testRun != "" {
		testArguments = append(testArguments, "-test.run", testRun)
	}

	testPackage, err := utils.GetMetadataAttribute("_test_package_name")
	if err != nil {
		log.Fatalf("failed to get metadata _test_package_name: %v", err)
	}

	workDir, err := ioutil.TempDir("", "image_test")
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
