package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"

	ghub "github.com/GoogleCloudPlatform/guest-test-infra/autoversioner/github"
	"github.com/GoogleCloudPlatform/guest-test-infra/autoversioner/version"
	"github.com/google/go-github/github"
)

var (
	tokenFile = flag.String("token-file-path", "", "path to github token file")
	org       = flag.String("org", "", "organization name")
	repo      = flag.String("repo", "", "repository name")
)

func main() {
	ctx := context.Background()
	flag.Parse()
	err := validateFlags()
	if err != nil {
		fmt.Printf("error validating flags: %+v", err)
		os.Exit(1)
	}

	buildID, err := generateVersion(ctx)
	if err != nil {
		fmt.Printf("error generating build id: %+v", err)
		os.Exit(1)
	}

	fmt.Printf("%s\n", buildID)
}

// gets the latest buildID
// fetches all the  tags associated with the org/repo
// sorts the matching versiontags in order of increasing order
// we have to do this because there is no way to get the latest
// version
func generateVersion(ctx context.Context) (string, error) {
	fetcher, err := ghub.NewClient(ctx, *tokenFile)
	if err != nil {
		return "", fmt.Errorf("error creating fetcher: %+v", err)
	}

	tags, err := fetcher.ListTags(ctx, *org, *repo)
	if err != nil {
		return "", fmt.Errorf("error fetching tags: %+v", err)
	}
	versions, err := getVersions(tags)

	lastVersion := version.Version{}
	// this repository is using our build and release pipeline
	// for the first time
	if len(versions) != 0 {
		sort.Sort(version.Sorter(versions))
		lastVersion = versions[len(versions)-1]
	}
	return lastVersion.IncrementVersion().String(), nil
}

// converts the github tag objects to Version objects
func getVersions(tags []*github.RepositoryTag) ([]version.Version, error) {
	if tags == nil || len(tags) == 0 {
		return nil, fmt.Errorf("invalid input")
	}

	var versions []version.Version
	for _, tag := range tags {
		v, err := version.NewVersion(*tag.Name)
		if err != nil {
			continue
		}
		versions = append(versions, *v)
	}

	return versions, nil
}

func validateFlags() error {
	if *tokenFile == "" {
		return fmt.Errorf("empty token file")
	}
	if *org == "" {
		return fmt.Errorf("empty org value")
	}
	if *repo == "" {
		return fmt.Errorf("empty repo value")
	}

	return nil
}
