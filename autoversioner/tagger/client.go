package tagger

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	TagObjectTypeCommit = "commit"
)

// Client is a tagger client
type Client struct {
	gc *github.Client
}

// NewClient returns a new tagger client
func NewClient(ctx context.Context, tokenFile string) (*Client, error) {
	if tokenFile == "" {
		return nil, fmt.Errorf("Unauthorized: no token file found")
	}
	b, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %v", tokenFile, err)
	}
	b = bytes.TrimSpace(b)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: string(b)})
	tc := oauth2.NewClient(ctx, ts)
	return &Client{github.NewClient(tc)}, nil
}

// CreateRef creates the given ref
// See https://developer.github.com/v3/git/refs/#create-a-reference
func (c *Client) CreateRef(ctx context.Context, owner, repo, ref, sha string) (*github.Reference, error) {
	reference := &github.Reference{
		Ref: &ref,
		Object: &github.GitObject{
			SHA: &sha,
		},
	}
	r, _, err := c.gc.Git.CreateRef(ctx, owner, repo, reference)
	return r, err
}

// CreateTag creates a tag in github repo
// https://developer.github.com/v3/git/tags/#create-a-tag-object
func (c *Client) CreateTag(ctx context.Context, org, repo, tag, sha, message, botUser, botEmail string) (github.Tag, error) {
	objectType := TagObjectTypeCommit
	t := github.Tag{
		Tag: &tag,
		Tagger: &github.CommitAuthor{
			Name:  &botUser,
			Email: &botEmail,
		},
		Object: &github.GitObject{
			SHA:  &sha,
			Type: &objectType,
		},
		Message: &message,
	}
	retTag, _, err := c.gc.Git.CreateTag(ctx, org, repo, &t)
	return *retTag, err
}
