package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"cloud.google.com/go/storage"
	"github.com/jstemmer/go-junit-report/formatter"
	"github.com/jstemmer/go-junit-report/parser"
)

const (
	metadataUrlPrefix   = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/"
	testTxtResult       = "/tmp/go-test.txt"
	testJunitResult     = "/tmp/junit_go-test.xml"
	testResultObject    = "outs/junit_go-test.xml"
	testBinaryLocalPath = "/tmp/image_test"
)

var testArgument = []string{"-test.v"}

func main() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create cloud storage client: %v\n", err)
	}

	testBinaryUrl, err := getMetadataAttribute("_test_binary_url")
	log.Printf("Metadata _test_binary_url: %v\n", testBinaryUrl)
	bucket, object, err := parseBucketObject(testBinaryUrl)
	if err != nil {
		log.Fatalf("Failed to pase gcs url: %v\n", err)
	}

	testRun, err := getMetadataAttribute("_test_run")
	log.Printf("Metadata _test_run: %v\n", testRun)
	if testRun != "" {
		testArgument = append(testArgument, "-test.run", testRun)
	}

	err = downloadObjectToFile(ctx, client, bucket, object, testBinaryLocalPath)
	if err != nil {
		log.Fatalf("Failed to download object: %v\n", err)
	}

	err = executeAndSaveOutput(testTxtResult, testBinaryLocalPath, testArgument...)
	if err != nil {
		log.Fatalf("Failed to execute test binary: %v\n", err)
	}

	err = covertTxtToJunit(testTxtResult, testJunitResult)
	if err != nil {
		log.Fatalf("Failed to convert to junit format: %v\n", err)
	}

	err = uploadObjectFromFile(ctx, client, bucket, testResultObject, testJunitResult)
	if err != nil {
		log.Fatalf("Failed to upload test result: %v\n", err)
	}
}

func parseBucketObject(gcsUrl string) (string, string, error) {
	u, err := url.Parse(gcsUrl)
	if err != nil {
		return "", "", fmt.Errorf("Failed to pase gcs url: %v\n", err)
	}
	return u.Host, u.Path, nil
}

func covertTxtToJunit(input, output string) error {
	in, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("Failed to open file %v", err)
	}

	out, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("Failed to create file %v", err)
	}
	report, err := parser.Parse(in, "")
	if err != nil {
		return err
	}
	err = formatter.JUnitReportXML(report, false, "", out)
	if err != nil {
		return err
	}
	return nil
}

func executeAndSaveOutput(file, name string, arg ...string) error {
	command := exec.Command(name, arg...)
	log.Printf("The command: %v\n", command.String())

	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("Failed to execute command: %v\n", err)
	}
	err = ioutil.WriteFile(file, output, 0755)
	if err != nil {
		return fmt.Errorf("Failed to write stdout: %v\n", err)
	}
	return nil
}

func getMetadataAttribute(attribute string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", metadataUrlPrefix, attribute), nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	val, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

func downloadObjectToFile(ctx context.Context, client *storage.Client, bucket, object, file string) error {
	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("Failed to open the reader: %v\n", err)
	}
	defer rc.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll: %v", err)
	}

	err = ioutil.WriteFile(file, data, 0777)
	if err != nil {
		return fmt.Errorf("Failed to write file: %v\n", err)
	}
	return nil
}

func uploadObjectFromFile(ctx context.Context, client *storage.Client, bucket, object, file string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("fail to open a file %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	wc := client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return fmt.Errorf("Failed to write file: %v", err)
	}
	wc.Close()
	return nil
}
