package github

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

const (
	treeBranch = "├──"
	treeLeaf   = "└──"
)

var _ release.Summarizer = (*Summarizer)(nil)

type Config struct {
	Host                   string
	IncludeIssuePRAuthors  bool
	IncludeIssues          bool
	IncludePRs             bool
	IssuesRequireLinkedPR  bool
	ExcludeLabels          []string
	ChangeTypesByLabel     change.TypeSet
	ConsiderPRMergeCommits bool
}

type Summarizer struct {
	git      git.Interface
	userName string
	repoName string
	config   Config
}

func NewSummarizer(gitter git.Interface, config Config) (*Summarizer, error) {
	repoURL, err := gitter.RemoteURL()
	if err != nil {
		return nil, err
	}

	user, repo := extractGithubUserAndRepo(repoURL)
	if user == "" || repo == "" {
		return nil, fmt.Errorf("failed to extract owner and repo from %q", repoURL)
	}

	log.WithFields("owner", user, "repo", repo).Debug("github summarizer")

	return &Summarizer{
		git:      gitter,
		userName: user,
		repoName: repo,
		config:   config,
	}, nil
}

func (s *Summarizer) Release(ref string) (*release.Release, error) {
	targetRelease, err := fetchRelease(s.userName, s.repoName, ref)
	if err != nil {
		return nil, err
	}
	if targetRelease.Tag == "" {
		return nil, nil
	}
	return &release.Release{
		Version: targetRelease.Tag,
		Date:    targetRelease.Date,
	}, nil
}

func (s *Summarizer) ReferenceURL(ref string) string {
	return fmt.Sprintf("https://%s/%s/%s/tree/%s", s.config.Host, s.userName, s.repoName, ref)
}

func (s *Summarizer) ChangesURL(sinceRef, untilRef string) string {
	return fmt.Sprintf("https://%s/%s/%s/compare/%s...%s", s.config.Host, s.userName, s.repoName, sinceRef, untilRef)
}

func (s *Summarizer) LastRelease() (*release.Release, error) {
	releases, err := fetchAllReleases(s.userName, s.repoName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch all releases: %v", err)
	}
	latestRelease := latestNonDraftRelease(releases)
	if latestRelease != nil {
		return &release.Release{
			Version: latestRelease.Tag,
			Date:    latestRelease.Date,
		}, nil
	}
	return nil, fmt.Errorf("unable to find latest release")
}

// nolint:funlen
func (s *Summarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	var changes []change.Change
	var err error

	var includeStart, includeEnd bool

	var sinceTag *git.Tag
	sinceHash := sinceRef
	if sinceRef != "" {
		sinceTag, err = s.git.SearchForTag(sinceRef)
		if err != nil {
			return nil, err
		}
		includeStart = false
	} else {
		includeStart = true
	}

	var untilTag *git.Tag
	untilHash := untilRef
	if untilRef != "" {
		untilTag, err = s.git.SearchForTag(untilRef)
		if err != nil {
			return nil, err
		}
		includeEnd = false
	} else {
		untilHash, err = s.git.HeadTagOrCommit()
		if err != nil {
			return nil, err
		}
		includeEnd = true
	}

	var includeCommits []string
	if s.config.ConsiderPRMergeCommits {
		includeCommits, err = s.git.CommitsBetween(git.Range{
			SinceRef:     sinceHash,
			UntilRef:     untilHash,
			IncludeStart: includeStart,
			IncludeEnd:   includeEnd,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to fetch commit range: %v", err)
		}

		log.Debugf("release comprised of %d commits", len(includeCommits))
		logCommits(includeCommits)
	}

	allMergedPRs, err := fetchMergedPRs(s.userName, s.repoName)
	if err != nil {
		return nil, err
	}

	log.Debugf("total merged PRs discovered: %d", len(allMergedPRs))

	if s.config.IncludePRs || (s.config.IssuesRequireLinkedPR && s.config.IncludeIssues) {
		if s.config.IncludePRs {
			changes = append(changes, changesFromStandardPRFilters(s.config, allMergedPRs, sinceTag, untilTag, includeCommits)...)
		}
		if s.config.IssuesRequireLinkedPR && s.config.IncludeIssues {
			// extract closed linked issues with closed PRs from the PR list. Why do this here?
			// githubs ontology has PRs as the source of truth for issue linking. Linked PR information
			// is not available on the issue itself.
			extractedIssues := issuesExtractedFromPRs(s.config, allMergedPRs, sinceTag, untilTag, includeCommits)
			changes = append(changes, createChangesFromIssues(s.config, allMergedPRs, extractedIssues, filterIssuesByChangeTypes)...)
		}
	}

	allClosedIssues, err := fetchClosedIssues(s.userName, s.repoName)
	if err != nil {
		return nil, err
	}

	log.Debugf("total closed issues discovered: %d", len(allClosedIssues))

	if s.config.IncludeIssues && !s.config.IssuesRequireLinkedPR {
		changes = append(changes, changesFromIssues(s.config, allMergedPRs, allClosedIssues, sinceTag, untilTag)...)
	}

	return changes, nil
}

func logCommits(commits []string) {
	for idx, commit := range commits {
		var branch = treeBranch
		if idx == len(commits)-1 {
			branch = treeLeaf
		}
		log.Tracef("  %s %s", branch, commit)
	}
}

func issuesExtractedFromPRs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag, includeCommits []string) []ghIssue {
	// this represents the traits we wish to filter down to (not out).
	prFilters := []prFilter{
		prsAfter(sinceTag.Timestamp.UTC()),
		// PRs with these labels should explicitly be used in the changelog directly (not the corresponding linked issue)
		prsWithoutLabel(config.ChangeTypesByLabel.Names()...),
		prsWithClosedLinkedIssue(),
	}

	if untilTag != nil {
		prFilters = append(prFilters, prsAtOrBefore(untilTag.Timestamp.UTC()))
	}

	includedPRs := applyPRFilters(allMergedPRs, config, sinceTag, untilTag, includeCommits, prFilters...)
	extractedIssues := uniqueIssuesFromPRs(includedPRs)

	// this represents the traits we wish to filter down to (not out).
	issueFilters := []issueFilter{
		issuesAfter(sinceTag.Timestamp),
		issuesWithLabel(config.ChangeTypesByLabel.Names()...),
		issuesWithoutLabel(config.ExcludeLabels...),
	}

	if untilTag != nil {
		issueFilters = append(issueFilters, issuesAtOrBefore(untilTag.Timestamp))
	}

	return filterIssues(extractedIssues, issueFilters...)
}

func uniqueIssuesFromPRs(prs []ghPullRequest) []ghIssue {
	issueNumbers := make(map[int]struct{})
	var issues []ghIssue
	for _, pr := range prs {
		for _, issue := range pr.LinkedIssues {
			if _, ok := issueNumbers[issue.Number]; ok {
				continue
			}
			issues = append(issues, issue)
			issueNumbers[issue.Number] = struct{}{}
		}
	}
	return issues
}

func changesFromStandardPRFilters(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag, includeCommits []string) []change.Change {
	includedPRs := applyStandardPRFilters(allMergedPRs, config, sinceTag, untilTag, includeCommits)

	includedPRs, _ = filterPRs(includedPRs, prsWithChangeTypes(config))

	log.Debugf("PRs contributing to changelog: %d", len(includedPRs))
	logPRs(includedPRs)

	return changesFromPRs(config, includedPRs)
}

func changesFromPRs(config Config, prs []ghPullRequest) []change.Change {
	var summaries []change.Change
	for _, pr := range prs {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(pr.Labels...)
		summaries = append(summaries, change.Change{
			Text:        pr.Title,
			ChangeTypes: changeTypes,
			Timestamp:   pr.MergedAt,
			References: []change.Reference{
				{
					Text: fmt.Sprintf("PR #%d", pr.Number),
					URL:  pr.URL,
				},
				{
					Text: pr.Author,
					URL:  fmt.Sprintf("https://%s/%s", config.Host, pr.Author),
				},
			},
			EntryType: "githubPR",
			Entry:     pr,
		})
	}
	return summaries
}

func logPRs(prs []ghPullRequest) {
	for idx, pr := range prs {
		var branch = treeBranch
		if idx == len(prs)-1 {
			branch = treeLeaf
		}
		log.Tracef("  %s #%d: merged %s", branch, pr.Number, internal.FormatDateTime(pr.MergedAt))
	}
}

func changesFromIssues(config Config, allMergedPRs []ghPullRequest, allClosedIssues []ghIssue, sinceTag, untilTag *git.Tag) []change.Change {
	filteredIssues := filterIssues(allClosedIssues, standardIssueFilters(config, sinceTag, untilTag)...)

	log.Debugf("issues contributing to changelog: %d", len(filteredIssues))
	logIssues(filteredIssues)

	return createChangesFromIssues(config, allMergedPRs, filteredIssues, filterIssuesByChangeTypes)
}

func filterIssuesByChangeTypes(config Config, issue ghIssue) bool {
	changeTypes := config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
	return len(changeTypes) > 0
}

func logIssues(issues []ghIssue) {
	for idx, issue := range issues {
		var branch = treeBranch
		if idx == len(issues)-1 {
			branch = treeLeaf
		}
		log.Tracef("  %s #%d: closed %s", branch, issue.Number, internal.FormatDateTime(issue.ClosedAt))
	}
}

func createChangesFromIssues(config Config, allMergedPRs []ghPullRequest, issues []ghIssue, filter func(Config, ghIssue) bool) (changes []change.Change) {
	for _, issue := range issues {
		if filter(config, issue) {
			changeTypes := config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)

			references := []change.Reference{
				{
					Text: fmt.Sprintf("Issue #%d", issue.Number),
					URL:  issue.URL,
				},
			}

			if config.IncludeIssuePRAuthors {
				for _, pr := range allMergedPRs {
					for _, linkedIssue := range pr.LinkedIssues {
						if linkedIssue.URL == issue.URL {
							references = append(references, change.Reference{
								Text: pr.Author,
								URL:  fmt.Sprintf("https://%s/%s", config.Host, pr.Author),
							})
						}
					}
				}
			}

			changes = append(changes, change.Change{
				Text:        issue.Title,
				ChangeTypes: changeTypes,
				Timestamp:   issue.ClosedAt,
				References:  references,
				EntryType:   "githubIssue",
				Entry:       issue,
			})
		}
	}
	return changes
}

func extractGithubUserAndRepo(u string) (string, string) {
	switch {
	// e.g. git@github.com:anchore/chronicle.git
	case strings.HasPrefix(u, "git@"):
		fields := strings.Split(u, ":")
		pair := strings.Split(fields[len(fields)-1], "/")

		if len(pair) != 2 {
			return "", ""
		}

		return pair[0], strings.TrimSuffix(pair[1], ".git")

	// https://github.com/anchore/chronicle.git
	case strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://"):
		urlObj, err := url.Parse(u)
		if err != nil {
			return "", ""
		}

		fields := strings.Split(strings.TrimPrefix(urlObj.Path, "/"), "/")

		if len(fields) != 2 {
			return "", ""
		}

		return fields[0], strings.TrimSuffix(fields[1], ".git")
	}
	return "", ""
}

func standardIssueFilters(config Config, sinceTag, untilTag *git.Tag) []issueFilter {
	// this represents the traits we wish to filter down to (not out).
	filters := []issueFilter{
		issuesWithLabel(config.ChangeTypesByLabel.Names()...),
		issuesWithoutLabel(config.ExcludeLabels...),
	}

	if sinceTag != nil {
		filters = append([]issueFilter{issuesAfter(sinceTag.Timestamp)}, filters...)
	}

	if untilTag != nil {
		filters = append([]issueFilter{issuesAtOrBefore(untilTag.Timestamp)}, filters...)
	}
	return filters
}

func standardQualitativePrFilters(config Config) []prFilter {
	// this represents the traits we wish to filter down to (not out).
	return []prFilter{
		prsWithLabel(config.ChangeTypesByLabel.Names()...),
		prsWithoutLabel(config.ExcludeLabels...),
		// Merged PRs linked to closed issues should be hidden so that the closed issue title takes precedence over the pr title
		prsWithoutClosedLinkedIssue(),
		// Merged PRs with open issues indicates a partial implementation. When the last PR is merged for the issue
		// then the feature should be included (by the pr, not the set of PRs)
		prsWithoutOpenLinkedIssue(),
	}
}

func standardChronologicalPrFilters(config Config, sinceTag, untilTag *git.Tag, commits []string) []prFilter {
	var filters []prFilter

	if config.ConsiderPRMergeCommits {
		filters = append(filters, prsWithoutMergeCommit(commits...))
	}

	if sinceTag != nil {
		filters = append([]prFilter{prsAfter(sinceTag.Timestamp.UTC())}, filters...)
	}

	if untilTag != nil {
		filters = append([]prFilter{prsAtOrBefore(untilTag.Timestamp.UTC())}, filters...)
	}
	return filters
}

func applyStandardPRFilters(allMergedPRs []ghPullRequest, config Config, sinceTag, untilTag *git.Tag, includeCommits []string, filters ...prFilter) []ghPullRequest {
	allFilters := standardQualitativePrFilters(config)
	filters = append(allFilters, filters...)
	return applyPRFilters(allMergedPRs, config, sinceTag, untilTag, includeCommits, filters...)
}
