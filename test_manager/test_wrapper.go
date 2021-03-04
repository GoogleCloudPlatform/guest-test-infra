package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/storage"
)

const metadata_url = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/"
const filePath = "test-result.txt"

func main() {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	daisy_outs_path, err := getMetadata("daisy-outs-path")
	test_binary, err := getMetadata("test_binary")
	run_argument, err := getMetadata("run_argument")
	if err != nil {
		log.Fatalf("Failed to get metadata: %v", err)
	}

	daisyBucket := strings.Replace(daisy_outs_path, "gs://", "", 1)
	daisyBucket = strings.Replace(daisyBucket, "/outs", "", 1)

	bytes, err := downloadObject(ctx, client, daisyBucket, "sources/"+test_binary)
	if err != nil {
		log.Fatalf("Failed to download object: %v", err)
	}
	err = os.WriteFile(test_binary, bytes, 0777)
	if err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}
	command := exec.Command("test_binary", "-test.run"+run_argument)

	output, err := command.Output()
	if err != nil {
		log.Fatalf("Failed to execute test binary: %v", err)
	}
	err = os.WriteFile(filePath, output, 0755)
	if err != nil {
		log.Fatalf("Failed to write test result: %v", err)
	}
	// TODO convert to junit format
	err = uploadObject(ctx, client, filePath, daisyBucket, "outs/e2e-test-result")
	if err != nil {
		log.Fatalf("Failed to upload test result: %v", err)
	}
}

func getMetadata(attribute string) (string, error) {
	req, err := http.NewRequest("GET", metadata_url+attribute, nil)
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
	return string(val), err
}

func downloadObject(ctx context.Context, client *storage.Client, bucket, object string) ([]byte, error) {
	rc, err := client.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer rc.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll: %v", err)
	}
	return data, nil
}

func uploadObject(ctx context.Context, client *storage.Client, file, bucket, object string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("fail to open a file %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	// Upload an object with storage.Writer.
	wc := client.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}
	return nil
}
