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

	"cloud.google.com/go/storage"
	junitFormatter "github.com/jstemmer/go-junit-report/formatter"
	junitParser "github.com/jstemmer/go-junit-report/parser"
)

const (
	metadataURLPrefix   = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/"
	testResultObject    = "outs/junit_go-test.xml"
	testBinaryLocalName = "image_test"
	workDir             = "/workspace/"
	artifactPath        = workDir + "artifact"
)

func main() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to create cloud storage client: %v", err)
	}

	testBinaryURL, err := getMetadataAttribute("_test_binary_url")
	if err != nil {
		log.Fatalf("failed to get metadata _test_binary_url: %v", err)
	}

	testRun, _ := getMetadataAttribute("_test_run")

	var testArguments = []string{"-test.v"}
	if testRun != "" {
		testArguments = append(testArguments, "-test.run", testRun)
	}

	if err = os.Mkdir(workDir, 0755); err != nil {
		log.Fatalf("failed to create work dir: %v", err)
	}
	if err = os.Mkdir(artifactPath, 0755); err != nil {
		log.Fatalf("failed to create artifact dir: %v", err)
	}
	if err = downloadGCSObject(ctx, client, testBinaryURL, workDir); err != nil {
		log.Fatalf("failed to download object: %v", err)
	}

	out, err := executeCMD(workDir+testBinaryLocalName, workDir, testArguments)
	if err != nil {
		log.Fatalf("failed to execute test binary: %v", err)
	}

	testData, err := convertTxtToJunit(out)
	if err != nil {
		log.Fatalf("failed to convert to junit format: %v", err)
	}

	if err = uploadGCSObject(ctx, client, testBinaryURL, testData); err != nil {
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

func executeCMD(cmd, dir string, arg []string) ([]byte, error) {
	command := exec.Command(cmd, arg...)
	command.Dir = dir
	log.Printf("The command: %v", command.String())

	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %v", err)
	}
	return output, nil
}

func getMetadataAttribute(attribute string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, metadataURLPrefix+attribute, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("http response code is %d", resp.StatusCode)
	}
	val, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func downloadGCSObject(ctx context.Context, client *storage.Client, testBinaryURL, workDir string) error {
	u, err := url.Parse(testBinaryURL)
	if err != nil {
		log.Fatalf("failed to parse gcs url: %v", err)
	}
	bucket, object := u.Host, u.Path

	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to open the reader: %v", err)
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll: %v", err)
	}

	if err = ioutil.WriteFile(workDir+testBinaryLocalName, data, 0755); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	return nil
}

func uploadGCSObject(ctx context.Context, client *storage.Client, testBinaryURL string, data io.Reader) error {
	u, err := url.Parse(testBinaryURL)
	if err != nil {
		log.Fatalf("failed to parse gcs url: %v", err)
	}
	bucket := u.Host
	dst := client.Bucket(bucket).Object(testResultObject).NewWriter(ctx)
	if _, err := io.Copy(dst, data); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	dst.Close()
	return nil
}
