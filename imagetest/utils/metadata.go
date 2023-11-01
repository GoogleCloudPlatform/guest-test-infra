package utils

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	metadataURLPrefix = "http://metadata.google.internal/computeMetadata/v1/"
)

var (
	// ErrMDSEntryNotFound is an error used to report 404 status code.
	ErrMDSEntryNotFound = errors.New("No metadata entry found: 404 error")
)

// GetMetadata does a HTTP Get request to the metadata server, the metadata entry of
// interest is provided by elem as the elements of the entry path, the following example
// does a Get request to the entry "instance/guest-attributes":
//
// resp, err := GetAttribute(context.Background(), "instance", "guest-attributes")
// ...
func GetMetadata(ctx context.Context, elem ...string) (string, error) {
	path, err := url.JoinPath(metadataURLPrefix, elem...)
	if err != nil {
		return "", fmt.Errorf("failed to parse metadata url: %+s", err)
	}

	body, _, err := doHTTPGet(ctx, path)
	return body, err
}

// GetMetadataWithHeaders is similar to GetMetadata it only differs on the return where GetMetadata
// returns only the response's body as a string and an error GetMetadataWithHeaders returns the
// response's body as a string, the headers and an error.
func GetMetadataWithHeaders(ctx context.Context, elem ...string) (string, http.Header, error) {
	path, err := url.JoinPath(metadataURLPrefix, elem...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse metadata url: %+s", err)
	}

	return doHTTPGet(ctx, path)
}

// PutMetadata does a HTTP Put request to the metadata server, the metadata entry of
// interest is provided by elem as the elements of the entry path, the following example
// does a Put request to the entry "instance/guest-attributes":
//
// err := PutMetadata(context.Background(), "instance", "guest-attributes")
// ...
func PutMetadata(ctx context.Context, elem ...string) error {
	path, err := url.JoinPath(metadataURLPrefix, elem...)
	if err != nil {
		return fmt.Errorf("failed to parse metadata url: %+v", err)
	}

	err = doHTTPPut(ctx, path)
	if err != nil {
		return err
	}

	return nil
}

func doHTTPRequest(req *http.Request) (*http.Response, error) {
	req.Header.Add("Metadata-Flavor", "Google")
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do the http request: %+v", err)
	}

	if resp.StatusCode == 404 {
		return nil, ErrMDSEntryNotFound
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http response code is %v", resp.StatusCode)
	}

	return resp, nil
}

func doHTTPGet(ctx context.Context, path string) (string, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create a http request with context: %+v", err)
	}

	resp, err := doHTTPRequest(req)
	if err != nil {
		return "", nil, err
	}

	val, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read http request body: %+v", err)
	}

	return string(val), resp.Header, nil
}

func doHTTPPut(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, path, nil)
	if err != nil {
		return fmt.Errorf("failed to create a http request with context: %+v", err)
	}

	_, err = doHTTPRequest(req)
	return err
}
