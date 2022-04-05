package github

import (
	"context"
	"os"
	"sort"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

type ghRelease struct {
	Tag      string
	Date     time.Time
	IsLatest bool
	IsDraft  bool
}

func latestNonDraftRelease(releases []ghRelease) *ghRelease {
	for i := len(releases) - 1; i >= 0; i-- {
		if !releases[i].IsDraft {
			return &releases[i]
		}
	}
	return nil
}

// nolint:funlen
func fetchAllReleases(user, repo string) ([]ghRelease, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	var allReleases []ghRelease

	// Query some details about a repository, an ghIssue in it, and its comments.
	{
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
				} `graphql:"releases(first:100, after:$releasesCursor)"`
			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

			RateLimit rateLimit
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String(user),
			"repositoryName":  githubv4.String(repo),
			"releasesCursor":  (*githubv4.String)(nil), // Null after argument to get first page.
		}

		// var limit rateLimit
		for {
			err := client.Query(context.Background(), &query, variables)
			if err != nil {
				return nil, err
			}
			// limit = query.RateLimit

			for _, iEdge := range query.Repository.Releases.Edges {
				allReleases = append(allReleases, ghRelease{
					Tag:      string(iEdge.Node.TagName),
					IsLatest: bool(iEdge.Node.IsLatest),
					IsDraft:  bool(iEdge.Node.IsDraft),
					Date:     iEdge.Node.PublishedAt.Time,
				})
			}

			if !query.Repository.Releases.PageInfo.HasNextPage {
				break
			}
			variables["releasesCursor"] = githubv4.NewString(query.Repository.Releases.PageInfo.EndCursor)
		}

		// for idx, is := range allReleases {
		//	fmt.Printf("%d: %+v\n", idx, is)
		//}
		// printJSON(limit)
	}

	sort.Slice(allReleases, func(i, j int) bool {
		return allReleases[i].Date.Before(allReleases[j].Date)
	})

	return allReleases, nil
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
		return nil, err
	}

	return &ghRelease{
		Tag:      string(query.Repository.Release.TagName),
		IsLatest: bool(query.Repository.Release.IsLatest),
		IsDraft:  bool(query.Repository.Release.IsDraft),
		Date:     query.Repository.Release.PublishedAt.Time,
	}, nil
}
