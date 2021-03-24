package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"

	"cloud.google.com/go/storage"
	junitFormatter "github.com/jstemmer/go-junit-report/formatter"
	junitParser "github.com/jstemmer/go-junit-report/parser"
)

const (
	metadataURLPrefix   = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/"
	testResultObject    = "outs/junit_go-test.xml"
	testBinaryLocalPath = "image_test"
	artifactPath        = "/artifact"
	workDir             = "/workspace"
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

	testRun, err := getMetadataAttribute("_test_run")

	var testArguments = []string{"-test.v"}
	if testRun != "" {
		testArguments = append(testArguments, "-test.run", testRun)
	}

	if err = os.Mkdir(workDir+artifactPath, 0755); err != nil {
		log.Fatalf("failed to create artifact dir: %v", err)
	}
	if err = os.Mkdir(workDir, 0755); err != nil {
		log.Fatalf("failed to create work dir: %v", err)
	}
	if err = os.Chdir(workDir); err != nil {
		log.Fatalf("failed to change work dir: %v", err)
	}
	if err = downloadGCSObject(ctx, client, testBinaryURL, testBinaryLocalPath); err != nil {
		log.Fatalf("failed to download object: %v", err)
	}

	out, err := executeCMD(testBinaryLocalPath, testArguments)
	if err != nil {
		log.Fatalf("failed to execute test binary: %v", err)
	}

	testData, err := convertTxtToJunit(out)
	if err != nil {
		log.Fatalf("failed to convert to junit format: %v", err)
	}

	if err = uploadGCSObject(ctx, client, bucket, testResultObject, testData); err != nil {
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

func executeCMD(cmd string, arg []string) ([]byte, error) {
	command := exec.Command(cmd, arg...)
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

func downloadGCSObject(ctx context.Context, client *storage.Client, testBinaryURL, file string) error {
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

	if err = ioutil.WriteFile(file, data, 0755); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	return nil
}

func uploadGCSObject(ctx context.Context, client *storage.Client, bucket, object string, data io.Reader) error {
	des := client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err := io.Copy(des, data); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	des.Close()
	return nil
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
