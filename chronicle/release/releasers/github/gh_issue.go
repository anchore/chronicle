package github

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/log"
)

type ghIssue struct {
	Title      string
	Number     int
	Author     string
	ClosedAt   time.Time
	Closed     bool
	NotPlanned bool
	Labels     []string
	URL        string
}

type issueFilter func(issue ghIssue) bool

func filterIssues(issues []ghIssue, filters ...issueFilter) []ghIssue {
	if len(filters) == 0 {
		return issues
	}

	results := make([]ghIssue, 0, len(issues))

issueLoop:
	for _, r := range issues {
		for _, f := range filters {
			if !f(r) {
				continue issueLoop
			}
		}
		results = append(results, r)
	}

	return results
}

//nolint:deadcode,unused
func issuesAtOrAfter(since time.Time) issueFilter {
	return func(issue ghIssue) bool {
		keep := issue.ClosedAt.After(since) || issue.ClosedAt.Equal(since)
		if !keep {
			log.Tracef("issue #%d filtered out: closed at or before %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

func issuesAtOrBefore(since time.Time) issueFilter {
	return func(issue ghIssue) bool {
		keep := issue.ClosedAt.Before(since) || issue.ClosedAt.Equal(since)
		if !keep {
			log.Tracef("issue #%d filtered out: closed at or after %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

func issuesAfter(since time.Time) issueFilter {
	return func(issue ghIssue) bool {
		keep := issue.ClosedAt.After(since)
		if !keep {
			log.Tracef("issue #%d filtered out: closed before %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

//nolint:deadcode,unused
func issuesBefore(since time.Time) issueFilter {
	return func(issue ghIssue) bool {
		keep := issue.ClosedAt.Before(since)
		if !keep {
			log.Tracef("issue #%d filtered out: closed after %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

func issuesWithLabel(labels ...string) issueFilter {
	return func(issue ghIssue) bool {
		for _, targetLabel := range labels {
			for _, l := range issue.Labels {
				if l == targetLabel {
					return true
				}
			}
		}

		log.Tracef("issue #%d filtered out: missing required label", issue.Number)

		return false
	}
}

func issuesWithoutLabel(labels ...string) issueFilter {
	return func(issue ghIssue) bool {
		for _, targetLabel := range labels {
			for _, l := range issue.Labels {
				if l == targetLabel {
					log.Tracef("issue #%d filtered out: has label %q", issue.Number, l)

					return false
				}
			}
		}
		return true
	}
}

func excludeIssuesNotPlanned(allMergedPRs []ghPullRequest) issueFilter {
	return func(issue ghIssue) bool {
		if issue.NotPlanned {
			if len(getLinkedPRs(allMergedPRs, issue)) > 0 {
				log.Tracef("issue #%d included: is closed as not planned but has linked PRs", issue.Number)
				return true
			}
			log.Tracef("issue #%d filtered out: as not planned", issue.Number)
			return false
		}
		return true
	}
}

func issuesWithChangeTypes(config Config) issueFilter {
	return func(issue ghIssue) bool {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
		keep := len(changeTypes) > 0
		if !keep {
			log.Tracef("issue #%d filtered out: no change types", issue.Number)
		}
		return keep
	}
}

func issuesWithoutLabels() issueFilter {
	return func(issue ghIssue) bool {
		keep := len(issue.Labels) == 0
		if !keep {
			log.Tracef("issue #%d filtered out: has labels", issue.Number)
		}
		return keep
	}
}

//nolint:funlen
func fetchClosedIssues(user, repo string) ([]ghIssue, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	var allIssues []ghIssue

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
				Issues     struct {
					PageInfo struct {
						EndCursor   githubv4.String
						HasNextPage bool
					}
					Edges []struct {
						Node struct {
							Title  githubv4.String
							Number githubv4.Int
							URL    githubv4.String
							Author struct {
								Login githubv4.String
							}
							Closed      githubv4.Boolean
							ClosedAt    githubv4.DateTime
							StateReason githubv4.String
							Labels      struct {
								Edges []struct {
									Node struct {
										Name githubv4.String
									}
								}
							} `graphql:"labels(first:100)"`
						}
					}
				} `graphql:"issues(first:100, states:CLOSED, after:$issuesCursor)"`
			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

			RateLimit rateLimit
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String(user),
			"repositoryName":  githubv4.String(repo),
			"issuesCursor":    (*githubv4.String)(nil), // Null after argument to get first page.
		}

		// var limit rateLimit
		for {
			err := client.Query(context.Background(), &query, variables)
			if err != nil {
				return nil, err
			}
			// limit = query.RateLimit

			for _, iEdge := range query.Repository.Issues.Edges {
				var labels []string
				for _, lEdge := range iEdge.Node.Labels.Edges {
					labels = append(labels, string(lEdge.Node.Name))
				}
				allIssues = append(allIssues, ghIssue{
					Title:      string(iEdge.Node.Title),
					Author:     string(iEdge.Node.Author.Login),
					ClosedAt:   iEdge.Node.ClosedAt.Time,
					Closed:     bool(iEdge.Node.Closed),
					Labels:     labels,
					URL:        string(iEdge.Node.URL),
					Number:     int(iEdge.Node.Number),
					NotPlanned: strings.EqualFold("NOT_PLANNED", string(iEdge.Node.StateReason)),
				})
			}

			if !query.Repository.Issues.PageInfo.HasNextPage {
				break
			}
			variables["issuesCursor"] = githubv4.NewString(query.Repository.Issues.PageInfo.EndCursor)
		}

		// for idx, is := range allIssues {
		//	fmt.Printf("%d: %+v\n", idx, is)
		//}
		// printJSON(limit)
	}

	return allIssues, nil
}
