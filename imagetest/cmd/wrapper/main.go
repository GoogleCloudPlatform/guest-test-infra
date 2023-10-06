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
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/guest-test-infra/imagetest/utils"
)

func main() {
	log.Printf("FINISHED-BOOTING")
	ctx := context.Background()
	skipOnBootAttributeKey := "skipOnBoot"
	rebootedNamespace := "reboot"
	rebootedKey := "rebooted"
	log.Printf("checking skipboot case")
	// Special case where we want to boot twice but skip running the wrapper on the first boot
	skipOnBoot, err := utils.GetMetadataAttribute(skipOnBootAttributeKey)
	if err != nil {
		log.Printf("did not find skip on boot metadata key %v", err)
	}
	err = utils.QueryMetadataGuestAttribute(ctx, rebootedNamespace, rebootedKey, http.MethodGet)
	log.Printf("rebooted error was %v", err)
	if skipOnBoot == "true" && err != nil {
		if err = utils.QueryMetadataGuestAttribute(ctx, rebootedNamespace, rebootedKey, http.MethodPut); err != nil {
			log.Printf("could not place guest attribute rebooted key")
		}
		os.Exit(0)
	}
	log.Printf("continuing past skipboot")
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create cloud storage client: %v", err)
	}
	defer func() {
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
