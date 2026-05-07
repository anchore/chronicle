package github

import (
	"errors"
	"fmt"

	"github.com/scylladb/go-set/strset"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

var _ release.TrunkSummarizer = (*Summarizer)(nil)

// Trunk produces commit-anchored release data for the trunk output format. The
// "kept vs filtered" disposition for each PR/issue is derived from the actual
// changelog pipeline (s.changes), so the trunk view always agrees with what
// the markdown encoders would produce — including issue-driven configurations
// where a PR contributes to the changelog via its closed linked issue rather
// than via its own labels.
func (s *Summarizer) Trunk(sinceRef, untilRef string) (*release.TrunkData, error) {
	scope, err := s.getChangeScope(sinceRef, untilRef)
	if err != nil {
		return nil, err
	}
	if scope == nil {
		return nil, errors.New("could not determine start/end of change range (no scope produced)")
	}

	// always fetch full commit metadata regardless of ConsiderPRMergeCommits
	commits, err := s.git.CommitsBetweenWithMeta(git.Range{
		SinceRef:     scope.Start.Ref,
		UntilRef:     scope.End.Ref,
		IncludeStart: scope.Start.Inclusive,
		IncludeEnd:   true,
	})
	if err != nil {
		return nil, err
	}

	log.WithFields("count", len(commits)).Debug("commits fetched for trunk")

	commitHashSet := strset.New()
	for _, c := range commits {
		commitHashSet.Add(c.Hash)
	}

	// run the full changelog pipeline so the trunk's kept/filtered decisions
	// match exactly what shows up in the markdown changelog
	keptChanges, err := s.changes(*scope)
	if err != nil {
		return nil, err
	}

	allMergedPRs, err := fetchMergedPRs(s.userName, s.repoName, scope.Start.Timestamp)
	if err != nil {
		return nil, err
	}

	log.WithFields("count", len(allMergedPRs), "since", scope.Start.Timestamp).Debug("merged PRs fetched for trunk")

	keptByCommit := mapKeptChangesToCommits(keptChanges, allMergedPRs)

	prMap := buildTrunkPRMap(s.config, allMergedPRs, commitHashSet, keptByCommit, scope.Start.Tag, scope.End.Tag)

	trunkCommits := make([]release.TrunkCommit, 0, len(commits))
	for _, c := range commits {
		tc := release.TrunkCommit{
			Hash:      c.Hash,
			URL:       fmt.Sprintf("https://%s/%s/%s/commit/%s", s.config.Host, s.userName, s.repoName, c.Hash),
			Subject:   c.Subject,
			Author:    c.Author,
			Timestamp: c.Timestamp,
		}
		if pr, ok := prMap[c.Hash]; ok {
			tc.PR = pr
		}
		trunkCommits = append(trunkCommits, tc)
	}

	return &release.TrunkData{Commits: trunkCommits}, nil
}

// mapKeptChangesToCommits builds a map from merge-commit hash to the kept
// changes that contributed via that commit. PR-changes map directly via
// MergeCommit; issue-changes are mapped by finding the PR that closed the
// issue (the issue's URL appears in the PR's LinkedIssues).
func mapKeptChangesToCommits(keptChanges []change.Change, allMergedPRs []ghPullRequest) map[string][]change.Change {
	out := make(map[string][]change.Change)
	for _, c := range keptChanges {
		commitHash := commitForKeptChange(c, allMergedPRs)
		if commitHash == "" {
			continue
		}
		out[commitHash] = append(out[commitHash], c)
	}
	return out
}

func commitForKeptChange(c change.Change, allMergedPRs []ghPullRequest) string {
	switch entry := c.Entry.(type) {
	case ghPullRequest:
		return entry.MergeCommit
	case ghIssue:
		for _, pr := range allMergedPRs {
			for _, linked := range pr.LinkedIssues {
				if linked.URL == entry.URL {
					return pr.MergeCommit
				}
			}
		}
	}
	return ""
}

// buildTrunkPRMap classifies all merged PRs whose MergeCommit is in the
// provided commit set, returning a map keyed by merge-commit hash. PRs outside
// the commit set are ignored entirely. A PR is "kept" if its commit appears in
// keptByCommit (meaning the PR contributed to the changelog directly or via a
// linked issue); otherwise a diagnostic reason is computed for display.
func buildTrunkPRMap(config Config, allMergedPRs []ghPullRequest, commitHashSet *strset.Set, keptByCommit map[string][]change.Change, sinceTag, untilTag *git.Tag) map[string]*release.TrunkPR {
	prMap := make(map[string]*release.TrunkPR)

	for _, pr := range allMergedPRs {
		if !commitHashSet.Has(pr.MergeCommit) {
			continue
		}

		kept := keptByCommit[pr.MergeCommit]
		if len(kept) > 0 {
			prMap[pr.MergeCommit] = buildKeptTrunkPR(config, pr, kept, sinceTag, untilTag)
			continue
		}

		prMap[pr.MergeCommit] = &release.TrunkPR{
			Number:   pr.Number,
			Title:    pr.Title,
			URL:      pr.URL,
			Author:   pr.Author,
			Labels:   pr.Labels,
			Filtered: true,
			Reason:   explainPRNotKept(pr, config, sinceTag, untilTag),
		}
	}

	return prMap
}

// buildKeptTrunkPR assembles a kept TrunkPR using change-types from whatever
// kept change(s) actually contributed to the changelog (could be the PR
// itself, or one or more linked issues, or both).
func buildKeptTrunkPR(config Config, pr ghPullRequest, keptForCommit []change.Change, sinceTag, untilTag *git.Tag) *release.TrunkPR {
	typeSet := make(map[string]change.Type)
	keptIssueURLs := make(map[string]bool)
	for _, kc := range keptForCommit {
		for _, t := range kc.ChangeTypes {
			typeSet[t.Name] = t
		}
		if issue, ok := kc.Entry.(ghIssue); ok {
			keptIssueURLs[issue.URL] = true
		}
	}

	changeTypes := make([]change.Type, 0, len(typeSet))
	for _, t := range typeSet {
		changeTypes = append(changeTypes, t)
	}

	return &release.TrunkPR{
		Number:      pr.Number,
		Title:       pr.Title,
		URL:         pr.URL,
		Author:      pr.Author,
		Labels:      pr.Labels,
		ChangeTypes: changeTypes,
		Issues:      buildKeptTrunkIssues(config, pr.LinkedIssues, keptIssueURLs, sinceTag, untilTag),
		Filtered:    false,
	}
}

// buildKeptTrunkIssues classifies each linked issue: kept if its URL is in
// keptURLs (meaning it contributed to the changelog), otherwise filtered with
// a diagnostic reason. Open linked issues are skipped — they don't ship.
func buildKeptTrunkIssues(config Config, linkedIssues []ghIssue, keptURLs map[string]bool, sinceTag, untilTag *git.Tag) []release.TrunkIssue {
	if len(linkedIssues) == 0 {
		return nil
	}

	result := make([]release.TrunkIssue, 0, len(linkedIssues))
	for _, issue := range linkedIssues {
		if !issue.Closed {
			continue
		}

		ti := release.TrunkIssue{
			Number: issue.Number,
			Title:  issue.Title,
			URL:    issue.URL,
			Labels: issue.Labels,
		}

		if keptURLs[issue.URL] {
			ti.ChangeTypes = config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
			result = append(result, ti)
			continue
		}

		ti.Filtered = true
		ti.Reason = explainIssueNotKept(issue, config, sinceTag, untilTag)
		result = append(result, ti)
	}
	return result
}

// explainPRNotKept runs the standard filter chain against a single PR and
// returns the first failure reason. When the PR has a closed linked issue
// (i.e., the contribution path is via the issue), the failure reason is
// derived from the issue filters and prefixed with "linked-issue:".
func explainPRNotKept(pr ghPullRequest, config Config, sinceTag, untilTag *git.Tag) string {
	for _, f := range standardChronologicalPrFilters(config, sinceTag, untilTag, nil) {
		var r string
		if !f(pr, &r) {
			return r
		}
	}

	exclFilter := prsWithoutLabel(config.ExcludeLabels...)
	var r string
	if !exclFilter(pr, &r) {
		return r
	}

	if hasClosedLinkedIssue(pr) {
		// expected contribution path is via the linked issue
		for _, linked := range pr.LinkedIssues {
			if !linked.Closed {
				continue
			}
			return "linked-issue:" + explainIssueNotKept(linked, config, sinceTag, untilTag)
		}
		return "linked-issue:not-included"
	}

	if !hasChangeTypeLabel(pr.Labels, config) {
		return "label:missing-required"
	}

	return "not-included"
}

// explainIssueNotKept runs the standard issue filter chain and returns the
// first failure reason.
func explainIssueNotKept(issue ghIssue, config Config, sinceTag, untilTag *git.Tag) string {
	for _, f := range standardIssueFilters(config, sinceTag, untilTag) {
		var r string
		if !f(issue, &r) {
			return r
		}
	}
	if !hasChangeTypeLabel(issue.Labels, config) {
		return "label:missing-required"
	}
	return "not-included"
}

func hasClosedLinkedIssue(pr ghPullRequest) bool {
	for _, linked := range pr.LinkedIssues {
		if linked.Closed {
			return true
		}
	}
	return false
}

func hasChangeTypeLabel(labels []string, config Config) bool {
	return len(config.ChangeTypesByLabel.ChangeTypes(labels...)) > 0
}
