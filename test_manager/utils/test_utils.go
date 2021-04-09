package utils

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/storage"
)

const metadataURLPrefix = "http://metadata.google.internal/computeMetadata/v1/instance/"

// GetRealVMName returns the real name of a VM running in the same test.
func GetRealVMName(name string) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(hostname, "-", 3)
	if len(parts) != 3 {
		return "", errors.New("hostname doesn't match scheme")
	}
	return strings.Join([]string{parts[0], name, parts[2]}, "-"), nil
}

// GetMetadataAttribute returns an attribute from metadata if present, and error if not.
func GetMetadataAttribute(attribute string) (string, error) {
	return GetMetadata("attributes/" + attribute)
}

// GetMetadata returns a metadata value for the specified key if it is present, and error if not.
func GetMetadata(path string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s%s", metadataURLPrefix, path), nil)
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
		return "", fmt.Errorf("http response code is %v", resp.StatusCode)
	}
	val, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// DownloadGCSObject downloads a GCS object.
func DownloadGCSObject(ctx context.Context, client *storage.Client, gcsPath string) ([]byte, error) {
	u, err := url.Parse(gcsPath)
	if err != nil {
		log.Fatalf("Failed to parse GCS url: %v\n", err)
	}
	object := strings.TrimPrefix(u.Path, "/")
	rc, err := client.Bucket(u.Host).Object(object).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// DownloadGCSObjectToFile downloads a GCS object, writing it to the specified file.
func DownloadGCSObjectToFile(ctx context.Context, client *storage.Client, gcsPath, file string) error {
	data, err := DownloadGCSObject(ctx, client, gcsPath)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(file, data, 0755); err != nil {
		return err
	}
	return nil
}
