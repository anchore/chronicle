package github

import (
	"fmt"
	"strings"

	"github.com/anchore/chronicle/internal/log"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
)

var _ release.Summarizer = (*ChangeSummarizer)(nil)

type Config struct {
	Host               string
	IncludeIssues      bool
	IncludePRs         bool
	ExcludeLabels      []string
	ChangeTypesByLabel change.TypeSet
}

type ChangeSummarizer struct {
	repoPath string
	userName string
	repoName string
	config   Config
}

func NewChangeSummarizer(path string, config Config) (*ChangeSummarizer, error) {
	repoURL, err := git.RemoteURL(path)
	if err != nil {
		return nil, err
	}

	user, repo := extractGithubUserAndRepo(repoURL)
	if user == "" || repo == "" {
		return nil, fmt.Errorf("failed to parse repo=%q URL", repoURL)
	}

	log.Debugf("github owner=%q repo=%q path=%q", user, repo, path)

	return &ChangeSummarizer{
		repoPath: path,
		userName: user,
		repoName: repo,
		config:   config,
	}, nil
}

func (s *ChangeSummarizer) Release(ref string) (*release.Release, error) {
	targetRelease, err := fetchRelease(s.userName, s.repoName, ref)
	if err != nil {
		return nil, err
	}
	return &release.Release{
		Version: targetRelease.Tag,
		Date:    targetRelease.Date,
	}, nil
}

func (s *ChangeSummarizer) TagURL(tag string) string {
	return fmt.Sprintf("https://%s/%s/%s/tree/%s", s.config.Host, s.userName, s.repoName, tag)
}

func (s *ChangeSummarizer) ChangesURL(sinceRef, untilRef string) string {
	return fmt.Sprintf("https://%s/%s/%s/compare/%s...%s", s.config.Host, s.userName, s.repoName, sinceRef, untilRef)
}

func (s *ChangeSummarizer) LastRelease() (*release.Release, error) {
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

func (s *ChangeSummarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	var changes []change.Change

	if s.config.IncludePRs {
		prChanges, err := s.changesFromPRs(sinceRef, untilRef)
		if err != nil {
			return nil, err
		}
		changes = append(changes, prChanges...)
	}

	if s.config.IncludeIssues {
		issueChanges, err := s.changesFromIssues(sinceRef, untilRef)
		if err != nil {
			return nil, err
		}
		changes = append(changes, issueChanges...)
	}

	return changes, nil
}

func (s *ChangeSummarizer) changesFromPRs(sinceRef, untilRef string) ([]change.Change, error) {
	allClosedPRs, err := fetchMergedPRs(s.userName, s.repoName)
	if err != nil {
		return nil, err
	}

	sinceTag, err := git.SearchForTag(s.repoPath, sinceRef)
	if err != nil {
		return nil, err
	}

	filters := []prFilter{
		prsAfter(sinceTag.Timestamp),
		prsWithLabel(s.config.ChangeTypesByLabel.Names()...),
		prsWithoutLabel(s.config.ExcludeLabels...),
		// Merged PRs linked to closed issues should be hidden so that the closed issue summary takes precedence
		prsWithClosedLinkedIssue(),
		// Merged PRs with open issues indicates a partial implementation. When the last PR is merged for the issue
		// the issue should be closed, then the feature should be included (by the issue, not the set of PRs)
		prsWithOpenLinkedIssue(),
	}

	if untilRef != "" {
		untilTag, err := git.SearchForTag(s.repoPath, untilRef)
		if err != nil {
			return nil, err
		}

		filters = append(filters, prsBefore(untilTag.Timestamp))
	}

	filteredPRs := filterPRs(allClosedPRs, filters...)

	var summaries []change.Change
	for _, pr := range filteredPRs {
		changeTypes := s.config.ChangeTypesByLabel.ChangeTypes(pr.Labels...)
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
						Text: fmt.Sprintf("%s", pr.Author),
						URL:  fmt.Sprintf("https://%s/%s", s.config.Host, pr.Author),
					},
				},
			})
		}
	}
	return summaries, nil
}

func (s *ChangeSummarizer) changesFromIssues(sinceRef, untilRef string) ([]change.Change, error) {
	allClosedIssues, err := fetchClosedIssues(s.userName, s.repoName)
	if err != nil {
		return nil, err
	}

	sinceTag, err := git.SearchForTag(s.repoPath, sinceRef)
	if err != nil {
		return nil, err
	}

	filters := []issueFilter{
		issuesAfter(sinceTag.Timestamp),
		issuesWithLabel(s.config.ChangeTypesByLabel.Names()...),
		issuesWithoutLabel(s.config.ExcludeLabels...),
	}

	if untilRef != "" {
		untilTag, err := git.SearchForTag(s.repoPath, untilRef)
		if err != nil {
			return nil, err
		}

		filters = append(filters, issuesBefore(untilTag.Timestamp))
	}

	filteredIssues := filterIssues(allClosedIssues, filters...)

	var summaries []change.Change
	for _, issue := range filteredIssues {
		changeTypes := s.config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
		if len(changeTypes) > 0 {
			summaries = append(summaries, change.Change{
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
			})
		}
	}
	return summaries, nil
}

// TODO: extract from multiple URL sources (not just git, e.g. git@github.com:someone/project.git... should at least support https)
// TODO: clean this up
func extractGithubUserAndRepo(url string) (string, string) {
	if !strings.HasPrefix(url, "git@") {
		return "", ""
	}
	fields := strings.Split(strings.TrimSuffix(url, ".git"), ":")
	pair := strings.Split(fields[len(fields)-1], "/")

	if len(pair) != 2 {
		return "", ""
	}

	return pair[0], pair[1]
}
