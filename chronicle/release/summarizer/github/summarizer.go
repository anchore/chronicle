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
	// TODO: add exclusions by label (e.g. if "wontfix" label exists, ignore other labels and don't include as a summary)
	for _, issue := range filteredIssues {
		changeTypes := s.config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)
		if len(changeTypes) > 0 {
			// TODO: make configurable that allows for adding summaries for non-categorized items
			summaries = append(summaries, change.Change{
				Text:        issue.Title,
				ChangeTypes: changeTypes,
				Timestamp:   issue.ClosedAt,
				References: []change.Reference{
					{
						Text: fmt.Sprintf("#%d", issue.Number),
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
