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
	"strings"

	"cloud.google.com/go/storage"
	"github.com/GoogleCloudPlatform/guest-test-infra/test_manager/utils"
	junitFormatter "github.com/jstemmer/go-junit-report/formatter"
	junitParser "github.com/jstemmer/go-junit-report/parser"
)

const (
	testBinaryLocalName = "image_test"
)

func main() {
	// These are placeholders until daisy supports guest attributes.
	log.Printf("FINISHED-BOOTING")
	defer func() { log.Printf("FINISHED-TEST") }()

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create cloud storage client: %v", err)
	}

	daisyOutsPath, err := utils.GetMetadataAttribute("daisy-outs-path")
	if err != nil {
		log.Fatalf("failed to get metadata _test_binary_url: %v", err)
	}
	daisyOutsPath = daisyOutsPath + "/"

	testBinaryURL, err := utils.GetMetadataAttribute("_test_binary_url")
	if err != nil {
		log.Fatalf("failed to get metadata _test_binary_url: %v", err)
	}

	testRun, _ := utils.GetMetadataAttribute("_test_run")

	var testArguments = []string{"-test.v"}
	if testRun != "" {
		testArguments = append(testArguments, "-test.run", testRun)
	}

	workDir, err := ioutil.TempDir("", "image_test")
	if err != nil {
		log.Fatalf("failed to create work dir: %v", err)
	}
	workDir = workDir + "/"

	if err = utils.DownloadGCSObjectToFile(ctx, client, testBinaryURL, workDir+testBinaryLocalName); err != nil {
		log.Fatalf("failed to download object: %v", err)
	}

	out, err := executeCmd(workDir+testBinaryLocalName, workDir, testArguments)
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			log.Printf("test binary exited with error: %v stderr: %q", ee, ee.Stderr)
		} else {
			log.Fatalf("failed to execute test binary: %v stdout: %q", err, out)
		}
	}

	log.Printf("command output:\n%s\n", out)

	testData, err := convertTxtToJunit(out)
	if err != nil {
		log.Fatalf("failed to convert to junit format: %v", err)
	}

	if err = uploadGCSObject(ctx, client, daisyOutsPath+"junit.xml", testData); err != nil {
		log.Fatalf("failed to upload test result: %v", err)
	}
}

func convertTxtToJunit(in []byte) (*bytes.Buffer, error) {
	var b bytes.Buffer
	r := bytes.NewReader(in)
	report, err := junitParser.Parse(r, "")
	if err != nil {
		return nil, err
	}
	if err = junitFormatter.JUnitReportXML(report, false, "", &b); err != nil {
		return nil, err
	}
	return &b, nil
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
