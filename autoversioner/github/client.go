package github

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	// GitObjectTypeCommit is commit type gitobject
	// refer to https://developer.github.com/v3/git/tags/#create-a-tag-object
	// parameter name: "type"
	GitObjectTypeCommit = "commit"
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
	objectType := GitObjectTypeCommit
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

// ListCommitsBetween fetches commits from a repository after a specific date
//refer: https://developer.github.com/v3/repos/commits/#list-commits-on-a-repository
func (c *Client) ListCommitsBetween(ctx context.Context, org, repo string, since, until time.Time) ([]*github.RepositoryCommit, error) {
	options := &github.CommitsListOptions{
		Since: since,
		Until: until,
	}
	commits, _, err := c.gc.Repositories.ListCommits(ctx, org, repo, options)
	return commits, err
}

// GetCommitBySha returns a commit for provided sha value
// refer: https://developer.github.com/v3/repos/commits/#get-a-single-commit
func (c *Client) GetCommitBySha(ctx context.Context, owner, repo, sha string) (*github.Commit, error) {
	commit, _, err := c.gc.Git.GetCommit(ctx, owner, repo, sha)
	return commit, err
}
