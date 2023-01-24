package github

import (
	"context"
	"os"
	"time"

	"github.com/scylladb/go-set/strset"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

type ghPullRequest struct {
	Title        string
	Number       int
	Author       string
	MergedAt     time.Time
	Labels       []string
	URL          string
	LinkedIssues []ghIssue
	MergeCommit  string
}

type prFilter func(issue ghPullRequest) bool

func applyPRFilters(allMergedPRs []ghPullRequest, config Config, sinceTag, untilTag *git.Tag, includeCommits []string, filters ...prFilter) []ghPullRequest {
	// first pass: exclude PRs which are not within the date range derived from the tags
	log.Trace("filtering PRs by chronology")
	includedPRs, excludedPRs := filterPRs(allMergedPRs, standardChronologicalPrFilters(config, sinceTag, untilTag, includeCommits)...)

	if config.ConsiderPRMergeCommits {
		// second pass: include PRs that are outside of the date range but have commits within what is considered for release explicitly
		log.Trace("considering re-inclusion of PRs based on merge commits")
		includedPRs = append(includedPRs, keepPRsWithCommits(excludedPRs, includeCommits)...)
	}

	// third pass: now that we have a list of PRs considered for release, we can filter down to those which have the correct traits (e.g. labels)
	log.Trace("filtering remaining PRs by qualitative traits")
	includedPRs, _ = filterPRs(includedPRs, filters...)

	return includedPRs
}

func filterPRs(prs []ghPullRequest, filters ...prFilter) ([]ghPullRequest, []ghPullRequest) {
	if len(filters) == 0 {
		return prs, nil
	}

	results := make([]ghPullRequest, 0, len(prs))
	removed := make([]ghPullRequest, 0, len(prs))

prLoop:
	for _, r := range prs {
		for _, f := range filters {
			if !f(r) {
				removed = append(removed, r)
				continue prLoop
			}
		}
		results = append(results, r)
	}

	return results, removed
}

// nolint:deadcode,unused
func prsAtOrAfter(since time.Time) prFilter {
	return func(pr ghPullRequest) bool {
		keep := pr.MergedAt.After(since) || pr.MergedAt.Equal(since)
		if !keep {
			log.Tracef("PR #%d filtered out: merged at or before %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

func prsAtOrBefore(since time.Time) prFilter {
	return func(pr ghPullRequest) bool {
		keep := pr.MergedAt.Before(since) || pr.MergedAt.Equal(since)
		if !keep {
			log.Tracef("PR #%d filtered out: merged at or after %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

func prsAfter(since time.Time) prFilter {
	return func(pr ghPullRequest) bool {
		keep := pr.MergedAt.After(since)
		if !keep {
			log.Tracef("PR #%d filtered out: merged before %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

// nolint:deadcode,unused
func prsBefore(since time.Time) prFilter {
	return func(pr ghPullRequest) bool {
		keep := pr.MergedAt.Before(since)
		if !keep {
			log.Tracef("PR #%d filtered out: merged after %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

func prsWithoutClosedLinkedIssue() prFilter {
	return func(pr ghPullRequest) bool {
		for _, i := range pr.LinkedIssues {
			if i.Closed {
				log.Tracef("PR #%d filtered out: has closed linked issue", pr.Number)
				return false
			}
		}
		return true
	}
}

func prsWithClosedLinkedIssue() prFilter {
	return func(pr ghPullRequest) bool {
		for _, i := range pr.LinkedIssues {
			if i.Closed {
				return true
			}
		}
		log.Tracef("PR #%d filtered out: does not have a closed linked issue", pr.Number)
		return false
	}
}

func prsWithoutOpenLinkedIssue() prFilter {
	return func(pr ghPullRequest) bool {
		for _, i := range pr.LinkedIssues {
			if !i.Closed {
				log.Tracef("PR #%d filtered out: has linked issue that is still open: issue %d", pr.Number, i.Number)

				return false
			}
		}
		return true
	}
}

func prsWithLabel(labels ...string) prFilter {
	return func(pr ghPullRequest) bool {
		for _, targetLabel := range labels {
			for _, l := range pr.Labels {
				if l == targetLabel {
					return true
				}
			}
		}
		log.Tracef("PR #%d filtered out: missing required label", pr.Number)

		return false
	}
}

func prsWithChangeTypes(config Config) prFilter {
	return func(pr ghPullRequest) bool {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(pr.Labels...)
		return len(changeTypes) > 0
	}
}

func prsWithoutLabel(labels ...string) prFilter {
	return func(pr ghPullRequest) bool {
		for _, targetLabel := range labels {
			for _, l := range pr.Labels {
				if l == targetLabel {
					log.Tracef("PR #%d filtered out: has label %q", pr.Number, l)
					return false
				}
			}
		}

		return true
	}
}

func prsWithoutMergeCommit(commits ...string) prFilter {
	commitSet := strset.New(commits...)
	return func(pr ghPullRequest) bool {
		if !commitSet.Has(pr.MergeCommit) {
			log.Tracef("PR #%d filtered out: has merge commit outside of valid set %s", pr.Number, pr.MergeCommit)
			return false
		}

		return true
	}
}

func keepPRsWithCommits(prs []ghPullRequest, commits []string, filters ...prFilter) []ghPullRequest {
	results := make([]ghPullRequest, 0, len(prs))

	commitSet := strset.New(commits...)
	for _, pr := range prs {
		if commitSet.Has(pr.MergeCommit) {
			log.Tracef("PR #%d included: has selected commit %s", pr.Number, pr.MergeCommit)
			keep, _ := filterPRs([]ghPullRequest{pr}, filters...)
			results = append(results, keep...)
		}
	}

	return results
}

// nolint:funlen
func fetchMergedPRs(user, repo string) ([]ghPullRequest, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	var allPRs []ghPullRequest

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
				DatabaseID   githubv4.Int
				URL          githubv4.URI
				PullRequests struct {
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
							MergeCommit struct {
								OID githubv4.String
							}
							MergedAt githubv4.DateTime
							Labels   struct {
								Edges []struct {
									Node struct {
										Name githubv4.String
									}
								}
							} `graphql:"labels(first:50)"`
							ClosingIssuesReferences struct {
								Nodes []struct {
									Title  githubv4.String
									Number githubv4.Int
									URL    githubv4.String
									Author struct {
										Login githubv4.String
									}
									ClosedAt githubv4.DateTime
									Closed   githubv4.Boolean
									Labels   struct {
										Edges []struct {
											Node struct {
												Name githubv4.String
											}
										}
									} `graphql:"labels(first:50)"`
								}
							} `graphql:"closingIssuesReferences(last:10)"`
						}
					}
				} `graphql:"pullRequests(first:100, states:MERGED, after:$prCursor)"`
			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

			RateLimit rateLimit
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String(user),
			"repositoryName":  githubv4.String(repo),
			"prCursor":        (*githubv4.String)(nil), // Null after argument to get first page.
		}

		// var limit rateLimit
		for {
			err := client.Query(context.Background(), &query, variables)
			if err != nil {
				return nil, err
			}
			// limit = query.RateLimit

			for _, prEdge := range query.Repository.PullRequests.Edges {
				var labels []string
				for _, lEdge := range prEdge.Node.Labels.Edges {
					labels = append(labels, string(lEdge.Node.Name))
				}

				var linkedIssues []ghIssue
				for _, iNodes := range prEdge.Node.ClosingIssuesReferences.Nodes {
					linkedIssues = append(linkedIssues, ghIssue{
						Title:    string(iNodes.Title),
						Author:   string(iNodes.Author.Login),
						ClosedAt: iNodes.ClosedAt.Time,
						Closed:   bool(iNodes.Closed),
						Labels:   labels,
						URL:      string(iNodes.URL),
						Number:   int(iNodes.Number),
					})
				}

				allPRs = append(allPRs, ghPullRequest{
					Title:        string(prEdge.Node.Title),
					Author:       string(prEdge.Node.Author.Login),
					MergedAt:     prEdge.Node.MergedAt.Time,
					Labels:       labels,
					URL:          string(prEdge.Node.URL),
					Number:       int(prEdge.Node.Number),
					LinkedIssues: linkedIssues,
					MergeCommit:  string(prEdge.Node.MergeCommit.OID),
				})
			}

			if !query.Repository.PullRequests.PageInfo.HasNextPage {
				break
			}
			variables["prCursor"] = githubv4.NewString(query.Repository.PullRequests.PageInfo.EndCursor)
		}
	}

	return allPRs, nil
}
