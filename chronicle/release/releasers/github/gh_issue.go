package github

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/anchore/chronicle/chronicle/event"
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

// issueFilter decides whether to keep an issue. When returning false, the optional
// ctx pointer (if non-nil) is populated with a short reason identifier.
type issueFilter func(issue ghIssue, ctx ...*string) bool

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

//nolint:unused
func issuesAtOrAfter(since time.Time) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		keep := issue.ClosedAt.After(since) || issue.ClosedAt.Equal(since)
		if !keep {
			setIssueReason(ctx, "chronology:before-since")
			log.Tracef("issue #%d filtered out: closed at or before %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

func issuesAtOrBefore(since time.Time) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		keep := issue.ClosedAt.Before(since) || issue.ClosedAt.Equal(since)
		if !keep {
			setIssueReason(ctx, "chronology:after-until")
			log.Tracef("issue #%d filtered out: closed at or after %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

func issuesAfter(since time.Time) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		keep := issue.ClosedAt.After(since)
		if !keep {
			setIssueReason(ctx, "chronology:before-since")
			log.Tracef("issue #%d filtered out: closed before %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

//nolint:unused
func issuesBefore(since time.Time) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		keep := issue.ClosedAt.Before(since)
		if !keep {
			setIssueReason(ctx, "chronology:after-until")
			log.Tracef("issue #%d filtered out: closed after %s (closed %s)", issue.Number, internal.FormatDateTime(since), internal.FormatDateTime(issue.ClosedAt))
		}
		return keep
	}
}

func issuesWithLabel(labels ...string) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		for _, targetLabel := range labels {
			for _, l := range issue.Labels {
				if l == targetLabel {
					return true
				}
			}
		}

		setIssueReason(ctx, "label:missing-required")
		log.Tracef("issue #%d filtered out: missing required label", issue.Number)

		return false
	}
}

func issuesWithoutLabel(labels ...string) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		for _, targetLabel := range labels {
			for _, l := range issue.Labels {
				if l == targetLabel {
					setIssueReason(ctx, "label:excluded:"+l)
					log.Tracef("issue #%d filtered out: has label %q", issue.Number, l)

					return false
				}
			}
		}
		return true
	}
}

func excludeIssuesNotPlanned(allMergedPRs []ghPullRequest) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		if issue.NotPlanned {
			if len(getLinkedPRs(allMergedPRs, issue)) > 0 {
				log.Tracef("issue #%d included: is closed as not planned but has linked PRs", issue.Number)
				return true
			}
			setIssueReason(ctx, "closed-as:not-planned")
			log.Tracef("issue #%d filtered out: as not planned", issue.Number)
			return false
		}
		return true
	}
}

func issuesWithChangeTypes(config Config) issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
		keep := len(changeTypes) > 0
		if !keep {
			setIssueReason(ctx, "change-type:none")
			log.Tracef("issue #%d filtered out: no change types", issue.Number)
		}
		return keep
	}
}

func issuesWithoutLabels() issueFilter {
	return func(issue ghIssue, ctx ...*string) bool {
		keep := len(issue.Labels) == 0
		if !keep {
			setIssueReason(ctx, "labels:present")
			log.Tracef("issue #%d filtered out: has labels", issue.Number)
		}
		return keep
	}
}

// setIssueReason writes the reason into the first element of the variadic ctx
// slice, if provided and non-nil.
func setIssueReason(ctx []*string, reason string) {
	if len(ctx) > 0 && ctx[0] != nil {
		*ctx[0] = reason
	}
}

//nolint:funlen
func fetchClosedIssues(user, repo string, since *time.Time, leaf *event.Leaf) ([]ghIssue, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	var (
		pages     = 1
		saw       = 0
		allIssues []ghIssue
	)

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
							UpdatedAt   githubv4.DateTime
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
				} `graphql:"issues(first:100, states:CLOSED, after:$issuesCursor, orderBy:{field: UPDATED_AT, direction: DESC})"`
			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

			RateLimit rateLimit
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String(user),
			"repositoryName":  githubv4.String(repo),
			"issuesCursor":    (*githubv4.String)(nil), // Null after argument to get first page.
		}

		// var limit rateLimit
		var (
			process   bool
			terminate = false
		)

		for !terminate {
			log.WithFields("user", user, "repo", repo, "page", pages).Trace("fetching closed issues from github.com")

			err := client.Query(context.Background(), &query, variables)
			if err != nil {
				return nil, explainGithubAPIError("query GitHub closed issues", user, repo, err)
			}

			// limit = query.RateLimit

			for i := range query.Repository.Issues.Edges {
				iEdge := query.Repository.Issues.Edges[i]
				saw++
				process, terminate = checkSearchTermination(since, &iEdge.Node.UpdatedAt, &iEdge.Node.ClosedAt)
				if !process || terminate {
					continue
				}

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

			// emit a stage update after each successfully fetched page so the UI
			// can show live progress on long fetches against big repos.
			leaf.SetStage(fmt.Sprintf("page %d — %d received", pages, saw))

			if !query.Repository.Issues.PageInfo.HasNextPage {
				break
			}
			variables["issuesCursor"] = githubv4.NewString(query.Repository.Issues.PageInfo.EndCursor)
			pages++
		}

		// for idx, is := range allIssues {
		//	fmt.Printf("%d: %+v\n", idx, is)
		//}
		// printJSON(limit)
	}

	log.WithFields("kept", len(allIssues), "saw", saw, "pages", pages, "since", since).Trace("closed PRs fetched from github.com")

	return allIssues, nil
}
