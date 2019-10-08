package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	ghub "github.com/GoogleCloudPlatform/guest-test-infra/autoversioner/github"
	"github.com/google/go-github/github"
)

var (
	tokenFile = flag.String("token-file-path", "", "path to github token file")
	org       = flag.String("org", "", "organization name")
	repo      = flag.String("repo", "", "repository name")
	sha       = flag.String("sha", "", "sha of commit id")
)

const (
	githubDateFormat = "20060102"
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

	fmt.Printf("Generated version: %s\n", buildID)
}

// generateVersion generates the version string.
// The algo to generate it is as follows:
// - for the given sha; fetch the date on which it was committed
// - fetch all the commits for that date,
// - in the list of commits of chronological order, find the
//   position of this commit
func generateVersion(ctx context.Context) (string, error) {
	generator, err := ghub.NewClient(ctx, *tokenFile)
	if err != nil {
		return "", fmt.Errorf("error creating generator: %+v", err)
	}
	commit, err := generator.GetCommitBySha(ctx, *org, *repo, *sha)
	if err != nil {
		return "", fmt.Errorf("error fetching git commit: %+v", err)
	}

	until := commit.Committer.Date
	since := getTodaysStartTime(*commit.Committer.Date)
	commits, err := generator.ListCommitsBetween(ctx, *org, *repo, *since, *until)

	if err != nil {
		return "", fmt.Errorf("error fetching commits for today: %+v", err)
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("Invalid commit sha")
	}

	idx, err := getIndex(*sha, commits)
	return fmt.Sprintf("%s.%02d", until.Format(githubDateFormat), len(commits)-idx-1), nil
}

func getIndex(sha string, commits []*github.RepositoryCommit) (int, error) {
	for idx, commit := range commits {
		if strings.Compare(commit.GetSHA(), sha) == 0 {
			return idx, nil
		}
	}
	return -1, fmt.Errorf("commit not found")
}

func getTodaysStartTime(t time.Time) *time.Time {
	since := t.Truncate(24 * time.Hour)
	return &since
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
	if *sha == "" {
		return fmt.Errorf("empty sha value")
	}

	return nil
}
