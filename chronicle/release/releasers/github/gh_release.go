package github

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type releaseFetcher func(user, repo, tag string) (*ghRelease, error)

type ghRelease struct {
	Tag      string
	Date     time.Time
	IsLatest bool
	IsDraft  bool
}

// fetchLatestNonDraftRelease returns the most recently created non-draft release for the given repo,
// or nil if the repo has no non-draft releases. It queries newest-first and returns on the first
// non-draft hit, only paginating if an entire page is drafts (rare in practice).
func fetchLatestNonDraftRelease(user, repo string) (*ghRelease, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	// TODO: act on hitting a rate limit
	type rateLimit struct {
		Cost      githubv4.Int
		Limit     githubv4.Int
		Remaining githubv4.Int
		ResetAt   githubv4.DateTime
	}

	var query struct {
		Repository struct {
			DatabaseID githubv4.Int
			URL        githubv4.URI
			Releases   struct {
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
				Edges []struct {
					Node struct {
						TagName     githubv4.String
						IsLatest    githubv4.Boolean
						IsDraft     githubv4.Boolean
						PublishedAt githubv4.DateTime
					}
				}
			} `graphql:"releases(first:100, after:$releasesCursor, orderBy:{field:CREATED_AT, direction:DESC})"`
		} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

		RateLimit rateLimit
	}
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(user),
		"repositoryName":  githubv4.String(repo),
		"releasesCursor":  (*githubv4.String)(nil), // null after argument to get first page.
	}

	for {
		err := client.Query(context.Background(), &query, variables)
		if err != nil {
			return nil, explainGithubAPIError("query GitHub releases", user, repo, err)
		}

		for _, edge := range query.Repository.Releases.Edges {
			if bool(edge.Node.IsDraft) {
				continue
			}
			return &ghRelease{
				Tag:      string(edge.Node.TagName),
				IsLatest: bool(edge.Node.IsLatest),
				IsDraft:  bool(edge.Node.IsDraft),
				Date:     edge.Node.PublishedAt.Time,
			}, nil
		}

		if !query.Repository.Releases.PageInfo.HasNextPage {
			return nil, nil
		}
		variables["releasesCursor"] = githubv4.NewString(query.Repository.Releases.PageInfo.EndCursor)
	}
}

func fetchRelease(user, repo, tag string) (*ghRelease, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)

	// TODO: act on hitting a rate limit
	type rateLimit struct {
		Cost      githubv4.Int
		Limit     githubv4.Int
		Remaining githubv4.Int
		ResetAt   githubv4.DateTime
	}

	var query struct {
		Repository struct {
			DatabaseID githubv4.Int
			URL        githubv4.URI
			Release    struct {
				TagName     githubv4.String
				IsLatest    githubv4.Boolean
				IsDraft     githubv4.Boolean
				PublishedAt githubv4.DateTime
			} `graphql:"release(tagName:$tagName)"`
		} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

		RateLimit rateLimit
	}
	variables := map[string]interface{}{
		"repositoryOwner": githubv4.String(user),
		"repositoryName":  githubv4.String(repo),
		"tagName":         githubv4.String(tag), // Null after argument to get first page.
	}

	err := client.Query(context.Background(), &query, variables)
	if err != nil {
		return nil, explainGithubAPIError(fmt.Sprintf("query GitHub release tag=%q", tag), user, repo, err)
	}

	return &ghRelease{
		Tag:      string(query.Repository.Release.TagName),
		IsLatest: bool(query.Repository.Release.IsLatest),
		IsDraft:  bool(query.Repository.Release.IsDraft),
		Date:     query.Repository.Release.PublishedAt.Time,
	}, nil
}
