package github

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/anchore/chronicle/internal/log"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
)

var _ release.Summarizer = (*Summarizer)(nil)

type Config struct {
	Host                  string
	IncludeIssues         bool
	IncludePRs            bool
	ExcludeLabels         []string
	ChangeTypesByLabel    change.TypeSet
	IssuesRequireLinkedPR bool
}

type Summarizer struct {
	repoPath string
	userName string
	repoName string
	config   Config
}

func NewSummarizer(path string, config Config) (*Summarizer, error) {
	repoURL, err := git.RemoteURL(path)
	if err != nil {
		return nil, err
	}

	user, repo := extractGithubUserAndRepo(repoURL)
	if user == "" || repo == "" {
		return nil, fmt.Errorf("failed to extract owner and repo from %q", repoURL)
	}

	log.Debugf("github owner=%q repo=%q path=%q", user, repo, path)

	return &Summarizer{
		repoPath: path,
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

func (s *Summarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	var changes []change.Change

	sinceTag, err := git.SearchForTag(s.repoPath, sinceRef)
	if err != nil {
		return nil, err
	}

	var untilTag *git.Tag
	if untilRef != "" {
		untilTag, err = git.SearchForTag(s.repoPath, untilRef)
		if err != nil {
			return nil, err
		}
	}

	if s.config.IncludePRs || (s.config.IssuesRequireLinkedPR && s.config.IncludeIssues) {
		allMergedPRs, err := fetchMergedPRs(s.userName, s.repoName)
		if err != nil {
			return nil, err
		}

		log.Debugf("total merged PRs discovered: %d", len(allMergedPRs))

		if s.config.IncludePRs {
			changes = append(changes, changesFromPRs(s.config, allMergedPRs, sinceTag, untilTag)...)
		}
		if s.config.IssuesRequireLinkedPR && s.config.IncludeIssues {
			// extract closed linked issues with closed PRs from the PR list. Why do this here?
			// githubs ontology has PRs as the source of truth for issue linking. Linked PR information
			// is not available on the issue itself.
			extractedIssues := issuesExtractedFromPRs(s.config, allMergedPRs, sinceTag, untilTag)
			changes = append(changes, createChangesFromIssues(s.config, extractedIssues)...)
		}
	}

	if s.config.IncludeIssues && !s.config.IssuesRequireLinkedPR {
		allClosedIssues, err := fetchClosedIssues(s.userName, s.repoName)
		if err != nil {
			return nil, err
		}

		log.Debugf("total closed issues discovered: %d", len(allClosedIssues))

		changes = append(changes, changesFromIssues(s.config, allClosedIssues, sinceTag, untilTag)...)
	}

	return changes, nil
}

func issuesExtractedFromPRs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag) []ghIssue {
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

	filteredPRs := filterPRs(allMergedPRs, prFilters...)
	extractedIssues := uniqueIssuesFromPRs(filteredPRs)

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

func changesFromPRs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag) []change.Change {
	filteredPRs := filterPRs(allMergedPRs, standardPrFilters(config, sinceTag, untilTag)...)

	log.Debugf("PRs contributing to changelog: %d", len(filteredPRs))

	var summaries []change.Change
	for _, pr := range filteredPRs {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(pr.Labels...)
		if len(changeTypes) > 0 {
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
	}
	return summaries
}

func changesFromIssues(config Config, allClosedIssues []ghIssue, sinceTag, untilTag *git.Tag) []change.Change {
	filteredIssues := filterIssues(allClosedIssues, standardIssueFilters(config, sinceTag, untilTag)...)

	log.Debugf("issues contributing to changelog: %d", len(filteredIssues))

	return createChangesFromIssues(config, filteredIssues)
}

func createChangesFromIssues(config Config, issues []ghIssue) (changes []change.Change) {
	for _, issue := range issues {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
		if len(changeTypes) > 0 {
			changes = append(changes, change.Change{
				Text:        issue.Title,
				ChangeTypes: changeTypes,
				Timestamp:   issue.ClosedAt,
				References: []change.Reference{
					{
						Text: fmt.Sprintf("Issue #%d", issue.Number),
						URL:  issue.URL,
					},
					// TODO: add assignee(s) name + url
				},
				EntryType: "githubIssue",
				Entry:     issue,
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
		issuesAfter(sinceTag.Timestamp),
		issuesWithLabel(config.ChangeTypesByLabel.Names()...),
		issuesWithoutLabel(config.ExcludeLabels...),
	}

	if untilTag != nil {
		filters = append(filters, issuesAtOrBefore(untilTag.Timestamp))
	}
	return filters
}

func standardPrFilters(config Config, sinceTag, untilTag *git.Tag) []prFilter {
	// this represents the traits we wish to filter down to (not out).
	filters := []prFilter{
		prsAfter(sinceTag.Timestamp.UTC()),
		prsWithLabel(config.ChangeTypesByLabel.Names()...),
		prsWithoutLabel(config.ExcludeLabels...),
		// Merged PRs linked to closed issues should be hidden so that the closed issue title takes precedence over the pr title
		prsWithoutClosedLinkedIssue(),
		// Merged PRs with open issues indicates a partial implementation. When the last PR is merged for the issue
		// then the feature should be included (by the pr, not the set of PRs)
		prsWithoutOpenLinkedIssue(),
	}

	if untilTag != nil {
		filters = append(filters, prsAtOrBefore(untilTag.Timestamp.UTC()))
	}
	return filters
}
