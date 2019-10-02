package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/guest-test-infra/autoversioner/tagger"
)

var (
	tokenFile = flag.String("token-file-path", "", "path to github token file")
	tag = flag.String("tag", "", "tag string to tag")
	org = flag.String("org", "", "organization name")
	repo = flag.String("repo", "", "repository name")
	sha = flag.String("sha", "", "sha of the github object to be tagged")
	message = flag.String("message", "", "message in the tag")

)

func main() {
	ctx := context.Background()
	flag.Parse()
	err := validateFlags()
	if err != nil {
		fmt.Printf("Error validating flags: %+v\n", err)
		os.Exit(1)
	}

	tagger, err := tagger.NewClient(ctx, *tokenFile)
	if err != nil {
		fmt.Printf("Error running tagger: %+v\n", err)
		os.Exit(1)
	}
	tag, err := tagger.CreateTag(ctx, *org, *repo, *tag, *sha, *message)
	if err != nil {
		fmt.Printf("Error while tagging: %+v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Added tag correctly: %+v\n", tag)
	// create ref /refs/tags/<tagname>; createtag just creates a tag object
	// refer https://developer.github.com/v3/git/tags/#create-a-tag-object
	ref, err := tagger.CreateRef(ctx, *org, *repo, fmt.Sprintf("refs/tags/%s", *tag.Tag), *tag.SHA)
	if err != nil {
		fmt.Printf("Error while creating ref: %+v", err)
		os.Exit(1)
	}
	fmt.Printf("added ref for tag: %+v\n", ref)

}

func validateFlags() error {
	if *tag == "" {
		return fmt.Errorf("Empty tag\n")
	}
	if *tokenFile == "" {
		return fmt.Errorf("empty token file\n")
	}
	if *org == "" {
		return fmt.Errorf("empty org value\n")
	}
	if *repo == "" {
		return fmt.Errorf("empty repo value\n")
	}
	if *sha == "" {
		return fmt.Errorf("empty sha value\n")
	}

	return nil

}