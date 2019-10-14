package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	ghub "github.com/GoogleCloudPlatform/guest-test-infra/autoversioner/github"
	"github.com/google/go-github/github"
)

var (
	tokenFile = flag.String("token-file-path", "", "path to github token file")
	tag       = flag.String("tag", "", "tag string to tag")
	org       = flag.String("org", "", "organization name")
	repo      = flag.String("repo", "", "repository name")
	sha       = flag.String("sha", "", "sha of the github object to be tagged")
	message   = flag.String("message", "", "message in the tag")
	botUser   = flag.String("bot-user-name", "guesttestinfra-bot", "github bot account id")
	botEmail  = flag.String("bot-email", "guesttestinfra-bot@google.com", "github bot account email id")
)

func main() {
	ctx := context.Background()
	flag.Parse()
	err := validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %+v\n", err)
		os.Exit(1)
	}

	ref, err := AddTag(ctx)
	if err != nil {
		fmt.Printf("Error creating tag: %+v\n", err)
		os.Exit(1)
	}
	fmt.Printf("added ref for tag: %+v\n", *ref)
}

// AddTag creates a ref to a github tag
// create ref /refs/tags/<tagname>; createtag just creates a tag object
// refer https://developer.github.com/v3/git/tags/#create-a-tag-object
func AddTag(ctx context.Context) (*github.Reference, error) {
	tagger, err := ghub.NewClient(ctx, *tokenFile)
	if err != nil {
		return nil, fmt.Errorf("error creating tagger: %+v", err)
	}
	tag, err := tagger.CreateTag(ctx, *org, *repo, *tag, *sha, *message, *botUser, *botEmail)
	if err != nil {
		return nil, fmt.Errorf("error while tagging: %+v", err)
	}
	fmt.Printf("Added tag correctly: %+v\n", tag)
	ref, err := tagger.CreateRef(ctx, *org, *repo, fmt.Sprintf("refs/tags/%s", *tag.Tag), *tag.SHA)
	if err != nil {
		return nil, fmt.Errorf("error while creating ref: %+v", err)
	}

	return ref, nil
}

func validateFlags() error {
	if *tag == "" {
		return fmt.Errorf("empty tag")
	}
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
