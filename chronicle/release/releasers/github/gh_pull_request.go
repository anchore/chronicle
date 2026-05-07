package github

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/scylladb/go-set/strset"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"

	"github.com/anchore/chronicle/chronicle/event"
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

// prFilter decides whether to keep a PR. When returning false, the optional ctx
// pointer (if non-nil) is populated with a short reason identifier.
type prFilter func(pr ghPullRequest, ctx ...*string) bool

// droppedPR pairs a PR with the reason it was filtered out.
type droppedPR struct {
	pr     ghPullRequest
	reason string
}

func applyPRFilters(allMergedPRs []ghPullRequest, config Config, sinceTag, untilTag *git.Tag, includeCommits []string, filters ...prFilter) []ghPullRequest {
	// first pass: exclude PRs which are not within the date range derived from the tags
	log.WithFields("input", len(allMergedPRs)).Trace("filtering PRs by chronology")
	includedPRs, excludedPRs := filterPRs(allMergedPRs, standardChronologicalPrFilters(config, sinceTag, untilTag, includeCommits)...)
	log.WithFields("kept", len(includedPRs), "dropped", len(excludedPRs)).Trace("PR chronology filter")

	if config.ConsiderPRMergeCommits {
		// second pass: include PRs that are outside of the date range but have commits within what is considered for release explicitly
		log.Trace("considering re-inclusion of PRs based on merge commits")
		reincluded := keepPRsWithCommits(excludedPRs, includeCommits)
		if len(reincluded) > 0 {
			log.WithFields("count", len(reincluded)).Trace("PRs re-included by merge commit")
		}
		includedPRs = append(includedPRs, prsFromDropped(reincluded)...)
	}

	// third pass: now that we have a list of PRs considered for release, we can filter down to those which have the correct traits (e.g. labels)
	beforeQual := len(includedPRs)
	includedPRs, droppedQual := filterPRs(includedPRs, filters...)
	log.WithFields("kept", len(includedPRs), "dropped", len(droppedQual), "input", beforeQual).Trace("PR qualitative filter")

	return includedPRs
}

// prsFromDropped extracts the underlying PR values from a dropped list.
func prsFromDropped(dropped []droppedPR) []ghPullRequest {
	prs := make([]ghPullRequest, len(dropped))
	for i, d := range dropped {
		prs[i] = d.pr
	}
	return prs
}

func filterPRs(prs []ghPullRequest, filters ...prFilter) (kept []ghPullRequest, dropped []droppedPR) {
	if len(filters) == 0 {
		return prs, nil
	}

	kept = make([]ghPullRequest, 0, len(prs))
	dropped = make([]droppedPR, 0, len(prs))

prLoop:
	for _, pr := range prs {
		for _, f := range filters {
			var reason string
			if !f(pr, &reason) {
				dropped = append(dropped, droppedPR{pr: pr, reason: reason})
				continue prLoop
			}
		}
		kept = append(kept, pr)
	}

	return kept, dropped
}

//nolint:unused
func prsAtOrAfter(since time.Time) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		keep := pr.MergedAt.After(since) || pr.MergedAt.Equal(since)
		if !keep {
			setReason(ctx, "chronology:too-old")
			log.Tracef("PR #%d filtered out: merged at or before %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

func prsAtOrBefore(since time.Time) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		keep := pr.MergedAt.Before(since) || pr.MergedAt.Equal(since)
		if !keep {
			setReason(ctx, "chronology:too-new")
			log.Tracef("PR #%d filtered out: merged at or after %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

func prsAfter(since time.Time) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		keep := pr.MergedAt.After(since)
		if !keep {
			setReason(ctx, "chronology:too-old")
			log.Tracef("PR #%d filtered out: merged before %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

//nolint:unused
func prsBefore(since time.Time) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		keep := pr.MergedAt.Before(since)
		if !keep {
			setReason(ctx, "chronology:too-new")
			log.Tracef("PR #%d filtered out: merged after %s (merged %s)", pr.Number, internal.FormatDateTime(since), internal.FormatDateTime(pr.MergedAt))
		}
		return keep
	}
}

func prsWithoutClosedLinkedIssue() prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		for _, i := range pr.LinkedIssues {
			if i.Closed {
				setReason(ctx, "linked-issue:closed")
				log.Tracef("PR #%d filtered out: has closed linked issue", pr.Number)
				return false
			}
		}
		return true
	}
}

func prsWithClosedLinkedIssue() prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		for _, i := range pr.LinkedIssues {
			if i.Closed {
				return true
			}
		}
		setReason(ctx, "linked-issue:none-closed")
		log.Tracef("PR #%d filtered out: does not have a closed linked issue", pr.Number)
		return false
	}
}

func prsWithoutOpenLinkedIssue() prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		for _, i := range pr.LinkedIssues {
			if !i.Closed {
				setReason(ctx, "linked-issue:open")
				log.Tracef("PR #%d filtered out: has linked issue that is still open: issue %d", pr.Number, i.Number)

				return false
			}
		}
		return true
	}
}

func prsWithLabel(labels ...string) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		for _, targetLabel := range labels {
			for _, l := range pr.Labels {
				if l == targetLabel {
					return true
				}
			}
		}
		setReason(ctx, "label:missing-required")
		log.Tracef("PR #%d filtered out: missing required label", pr.Number)

		return false
	}
}

func prsWithoutLabels() prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		keep := len(pr.Labels) == 0
		if !keep {
			setReason(ctx, "labels:present")
			log.Tracef("PR #%d filtered out: has labels", pr.Number)
		}
		return keep
	}
}

func prsWithoutLinkedIssues() prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		keep := len(pr.LinkedIssues) == 0
		if !keep {
			setReason(ctx, "linked-issues:present")
			log.Tracef("PR #%d filtered out: has linked issues", pr.Number)
		}
		return keep
	}
}

func prsWithChangeTypes(config Config) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(pr.Labels...)

		keep := len(changeTypes) > 0
		if !keep {
			setReason(ctx, "change-type:none")
			log.Tracef("PR #%d filtered out: no change types", pr.Number)
		}
		return keep
	}
}

func prsWithoutLabel(labels ...string) prFilter {
	return func(pr ghPullRequest, ctx ...*string) bool {
		for _, targetLabel := range labels {
			for _, l := range pr.Labels {
				if l == targetLabel {
					setReason(ctx, "label:excluded:"+l)
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
	return func(pr ghPullRequest, ctx ...*string) bool {
		if !commitSet.Has(pr.MergeCommit) {
			setReason(ctx, "merge-commit:not-in-set")
			log.Tracef("PR #%d filtered out: has merge commit outside of valid set %s", pr.Number, pr.MergeCommit)
			return false
		}

		return true
	}
}

func keepPRsWithCommits(dropped []droppedPR, commits []string, filters ...prFilter) []droppedPR {
	results := make([]droppedPR, 0, len(dropped))

	commitSet := strset.New(commits...)
	for _, d := range dropped {
		pr := d.pr
		if commitSet.Has(pr.MergeCommit) {
			log.Tracef("PR #%d included: has selected commit %s", pr.Number, pr.MergeCommit)
			keep, _ := filterPRs([]ghPullRequest{pr}, filters...)
			for _, kpr := range keep {
				results = append(results, droppedPR{pr: kpr, reason: ""})
			}
		} else {
			log.Tracef("PR #%d filtered out: does not have merge commit %s", pr.Number, pr.MergeCommit)
		}
	}

	return results
}

// setReason writes the reason into the first element of the variadic ctx slice,
// if provided and non-nil. This allows filter callers to retrieve the drop reason.
func setReason(ctx []*string, reason string) {
	if len(ctx) > 0 && ctx[0] != nil {
		*ctx[0] = reason
	}
}

//nolint:funlen
func fetchMergedPRs(user, repo string, since *time.Time, leaf *event.Leaf) ([]ghPullRequest, error) {
	src := oauth2.StaticTokenSource(
		// TODO: DI this
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	httpClient := oauth2.NewClient(context.Background(), src)
	client := githubv4.NewClient(httpClient)
	var (
		pages  = 1
		saw    = 0
		allPRs []ghPullRequest
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
							UpdatedAt githubv4.DateTime
							MergedAt  githubv4.DateTime
							Labels    struct {
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
				} `graphql:"pullRequests(first:100, states:MERGED, after:$prCursor, orderBy:{field: UPDATED_AT, direction: DESC})"`
			} `graphql:"repository(owner:$repositoryOwner, name:$repositoryName)"`

			RateLimit rateLimit
		}
		variables := map[string]interface{}{
			"repositoryOwner": githubv4.String(user),
			"repositoryName":  githubv4.String(repo),
			"prCursor":        (*githubv4.String)(nil), // Null after argument to get first page.
		}

		// var limit rateLimit
		var (
			process   bool
			terminate = false
		)

		for !terminate {
			log.WithFields("user", user, "repo", repo, "page", pages).Trace("fetching merged PRs from github.com")

			err := client.Query(context.Background(), &query, variables)
			if err != nil {
				return nil, explainGithubAPIError("query GitHub merged PRs", user, repo, err)
			}

			// limit = query.RateLimit

			for i := range query.Repository.PullRequests.Edges {
				prEdge := query.Repository.PullRequests.Edges[i]
				saw++
				process, terminate = checkSearchTermination(since, &prEdge.Node.UpdatedAt, &prEdge.Node.MergedAt)
				if !process || terminate {
					continue
				}

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

			// emit a stage update after each successfully fetched page so the UI
			// can show live progress on long fetches against big repos.
			leaf.SetStage(fmt.Sprintf("page %d — %d received", pages, saw))

			if !query.Repository.PullRequests.PageInfo.HasNextPage {
				break
			}
			variables["prCursor"] = githubv4.NewString(query.Repository.PullRequests.PageInfo.EndCursor)
			pages++
		}
	}

	log.WithFields("kept", len(allPRs), "saw", saw, "pages", pages, "since", since).Trace("merged PRs fetched from github.com")

	return allPRs, nil
}

func checkSearchTermination(since *time.Time, updatedAt, closedAt *githubv4.DateTime) (process bool, terminate bool) {
	process = true
	if since == nil {
		return
	}

	if closedAt.Before(*since) {
		process = false
	}

	if updatedAt.Before(*since) {
		terminate = true
	}

	return
}
