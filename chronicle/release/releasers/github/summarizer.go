package github

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/go-logger"
)

const (
	treeBranch = "├──"
	treeLeaf   = "└──"
)

var _ release.Summarizer = (*Summarizer)(nil)

type Config struct {
	Host                            string
	IncludeIssuePRAuthors           bool
	IncludeIssues                   bool
	IncludeIssuePRs                 bool
	IncludeIssuesClosedAsNotPlanned bool
	IncludePRs                      bool
	IncludeUnlabeledIssues          bool
	IncludeUnlabeledPRs             bool
	ExcludeLabels                   []string
	ChangeTypesByLabel              change.TypeSet
	IssuesRequireLinkedPR           bool
	ConsiderPRMergeCommits          bool
}

type Summarizer struct {
	git            git.Interface
	userName       string
	repoName       string
	config         Config
	releaseFetcher releaseFetcher
}

// changeScope is used to describe the start and end of a changes made in a repo.
type changeScope struct {
	Commits []string
	Start   changePoint
	End     changePoint
}

// changePoint is a single point on the timeline of changes in a repo.
type changePoint struct {
	Ref       string
	Tag       *git.Tag
	Inclusive bool
	Timestamp *time.Time
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
		git:            gitter,
		userName:       user,
		repoName:       repo,
		config:         config,
		releaseFetcher: fetchRelease,
	}, nil
}

func (s *Summarizer) Release(ref string) (*release.Release, error) {
	targetRelease, err := s.releaseFetcher(s.userName, s.repoName, ref)
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

func (s *Summarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	scope, err := s.getChangeScope(sinceRef, untilRef)
	if err != nil {
		return nil, err
	}

	if scope == nil {
		return nil, fmt.Errorf("unable to find start and end of changes: %w", err)
	}

	logChangeScope(*scope, s.config.ConsiderPRMergeCommits)

	return s.changes(*scope)
}

func (s *Summarizer) getChangeScope(sinceRef, untilRef string) (*changeScope, error) {
	sinceTag, sinceRef, includeStart, sinceTime, err := s.getSince(sinceRef)
	if err != nil {
		return nil, err
	}

	var untilTag *git.Tag
	var untilTime *time.Time
	if untilRef != "" {
		untilTag, err = s.git.SearchForTag(untilRef)
		if err != nil {
			return nil, fmt.Errorf("unable to find git tag %q: %w", untilRef, err)
		}
		untilTime = &untilTag.Timestamp
	} else {
		untilRef, err = s.git.HeadTagOrCommit()
		if err != nil {
			return nil, fmt.Errorf("unable to find git head reference: %w", err)
		}
	}

	var includeCommits []string
	if s.config.ConsiderPRMergeCommits {
		includeCommits, err = s.git.CommitsBetween(git.Range{
			SinceRef:     sinceRef,
			UntilRef:     untilRef,
			IncludeStart: includeStart,
			IncludeEnd:   true,
		})
		if err != nil {
			return nil, fmt.Errorf("unable to fetch commit range: %v", err)
		}
	}

	return &changeScope{
		Commits: includeCommits,
		Start: changePoint{
			Ref:       sinceRef,
			Tag:       sinceTag,
			Inclusive: includeStart,
			Timestamp: sinceTime,
		},
		End: changePoint{
			Ref:       untilRef,
			Tag:       untilTag,
			Inclusive: true,
			Timestamp: untilTime,
		},
	}, nil
}

func (s *Summarizer) getSince(sinceRef string) (*git.Tag, string, bool, *time.Time, error) {
	var err error
	var sinceTag *git.Tag
	var includeStart bool

	if sinceRef != "" {
		sinceTag, err = s.git.SearchForTag(sinceRef)
		if err != nil {
			return nil, "", false, nil, fmt.Errorf("unable to find git tag %q: %w", sinceRef, err)
		}
	}

	var sinceTime *time.Time
	if sinceTag != nil {
		sinceRelease, err := s.releaseFetcher(s.userName, s.repoName, sinceTag.Name)
		if err != nil {
			return nil, "", false, nil, fmt.Errorf("unable to fetch release %q: %w", sinceTag.Name, err)
		}
		if sinceRelease != nil {
			sinceTime = &sinceRelease.Date
		}
	}

	if sinceTag == nil {
		sinceRef, err = s.git.FirstCommit()
		if err != nil {
			return nil, "", false, nil, fmt.Errorf("unable to find first commit: %w", err)
		}
		includeStart = true
	}

	return sinceTag, sinceRef, includeStart, sinceTime, nil
}

func (s *Summarizer) changes(scope changeScope) ([]change.Change, error) {
	var changes []change.Change

	allMergedPRs, err := fetchMergedPRs(s.userName, s.repoName, scope.Start.Timestamp)
	if err != nil {
		return nil, err
	}

	log.WithFields("since", scope.Start.Timestamp).Debugf("total merged PRs discovered: %d", len(allMergedPRs))

	if s.config.IncludePRs {
		changes = append(changes, changesFromStandardPRFilters(s.config, allMergedPRs, scope.Start.Tag, scope.End.Tag, scope.Commits)...)
	}

	allClosedIssues, err := fetchClosedIssues(s.userName, s.repoName, scope.Start.Timestamp)
	if err != nil {
		return nil, err
	}

	if !s.config.IncludeIssuesClosedAsNotPlanned {
		allClosedIssues = filterIssues(allClosedIssues, excludeIssuesNotPlanned(allMergedPRs))
	}

	log.WithFields("since", scope.Start.Timestamp).Debugf("total closed issues discovered: %d", len(allClosedIssues))

	if s.config.IncludeIssues {
		if s.config.IssuesRequireLinkedPR {
			changes = append(changes, changesFromIssuesLinkedToPrs(s.config, allMergedPRs, scope.Start.Tag, scope.End.Tag, scope.Commits)...)
		} else {
			changes = append(changes, changesFromIssues(s.config, allMergedPRs, allClosedIssues, scope.Start.Tag, scope.End.Tag)...)
		}
	}

	if s.config.IncludeUnlabeledIssues {
		changes = append(changes, changesFromUnlabeledIssues(s.config, allMergedPRs, allClosedIssues, scope.Start.Tag, scope.End.Tag)...)
	}

	if s.config.IncludeUnlabeledPRs {
		changes = append(changes, changesFromUnlabeledPRs(s.config, allMergedPRs, scope.Start.Tag, scope.End.Tag, scope.Commits)...)
	}

	return changes, nil
}

func logChangeScope(c changeScope, considerCommits bool) {
	log.WithFields("since", c.Start.Ref, "until", c.End.Ref).Info("searching for changes")
	log.WithFields(changePointFields(c.Start)).Debug("  ├── since")
	log.WithFields(changePointFields(c.End)).Debug("  └── until")

	if considerCommits {
		log.Debugf("release comprises %d commits", len(c.Commits))
		logCommits(c.Commits)
	}

	// in a release process there tends to be a start point that is a github release and an end point that is a git tag.
	// in cases where the git tag is a lightweight tag encourage users to migrate to using annotated tags since
	// the annotated tag will have a timestamp associated with when the tag action was done and not when the PR merge
	// to main was done.
	if c.End.Tag != nil && !c.End.Tag.Annotated {
		log.WithFields("tag", c.End.Tag.Name).Warn("use of a lightweight git tag found, use annotated git tags for more accurate results")
	}
}

func changePointFields(p changePoint) logger.Fields {
	fields := logger.Fields{}
	if p.Tag != nil {
		fields["tag"] = p.Tag.Name
		fields["commit"] = p.Tag.Commit
	} else if p.Ref != "" {
		fields["commit"] = p.Ref
	}
	fields["inclusive"] = p.Inclusive
	if p.Timestamp != nil {
		fields["timestamp"] = internal.FormatDateTime(*p.Timestamp)
	}

	return fields
}

func logCommits(commits []string) {
	for idx, commit := range commits {
		var branch = treeBranch
		if idx == len(commits)-1 {
			branch = treeLeaf
		}
		log.Debugf("  %s %s", branch, commit)
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

	return createChangesFromPRs(config, includedPRs)
}

func createChangesFromPRs(config Config, prs []ghPullRequest) []change.Change {
	var summaries []change.Change
	for _, pr := range prs {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(pr.Labels...)

		if len(changeTypes) == 0 {
			changeTypes = change.UnknownTypes
		}

		summaries = append(summaries, change.Change{
			Text:        pr.Title,
			ChangeTypes: changeTypes,
			Timestamp:   pr.MergedAt,
			References: []change.Reference{
				{
					Text: fmt.Sprintf("#%d", pr.Number),
					URL:  pr.URL,
				},
				{
					Text: fmt.Sprintf("@%s", pr.Author),
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

func changesFromIssuesLinkedToPrs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag, includeCommits []string) []change.Change {
	// extract closed linked issues with closed PRs from the PR list. Why do this here?
	// githubs ontology has PRs as the source of truth for issue linking. Linked PR information
	// is not available on the issue itself.
	issues := issuesExtractedFromPRs(config, allMergedPRs, sinceTag, untilTag, includeCommits)
	issues = filterIssues(issues, issuesWithChangeTypes(config))

	log.Debugf("linked issues contributing to changelog: %d", len(issues))
	logIssues(issues)

	return createChangesFromIssues(config, allMergedPRs, issues)
}

func changesFromIssues(config Config, allMergedPRs []ghPullRequest, allClosedIssues []ghIssue, sinceTag, untilTag *git.Tag) []change.Change {
	filteredIssues := filterIssues(allClosedIssues, standardIssueFilters(config, sinceTag, untilTag)...)

	filteredIssues = filterIssues(filteredIssues, issuesWithChangeTypes(config))

	log.Debugf("issues contributing to changelog: %d", len(filteredIssues))
	logIssues(filteredIssues)

	return createChangesFromIssues(config, allMergedPRs, filteredIssues)
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

func changesFromUnlabeledPRs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag, includeCommits []string) []change.Change {
	// this represents the traits we wish to filter down to (not out).
	filters := []prFilter{
		prsWithoutLabels(),
		prsWithoutLinkedIssues(),
	}

	filters = append(filters, standardChronologicalPrFilters(config, sinceTag, untilTag, includeCommits)...)

	filteredIssues, _ := filterPRs(allMergedPRs, filters...)

	log.Debugf("unlabeled PRs contributing to changelog: %d", len(filteredIssues))

	return createChangesFromPRs(config, filteredIssues)
}

func changesFromUnlabeledIssues(config Config, allMergedPRs []ghPullRequest, allIssues []ghIssue, sinceTag, untilTag *git.Tag) []change.Change {
	// this represents the traits we wish to filter down to (not out).
	filters := standardChronologicalIssueFilters(sinceTag, untilTag)

	filters = append(filters, issuesWithoutLabels())

	filteredIssues := filterIssues(allIssues, filters...)

	log.Debugf("unlabeled issues contributing to changelog: %d", len(filteredIssues))

	return createChangesFromIssues(config, allMergedPRs, filteredIssues)
}

func createChangesFromIssues(config Config, allMergedPRs []ghPullRequest, issues []ghIssue) (changes []change.Change) {
	for _, issue := range issues {
		changeTypes := config.ChangeTypesByLabel.ChangeTypes(issue.Labels...)

		if len(changeTypes) == 0 {
			changeTypes = change.UnknownTypes
		}

		references := []change.Reference{
			{
				Text: fmt.Sprintf("#%d", issue.Number),
				URL:  issue.URL,
			},
		}

		if config.IncludeIssuePRs || config.IncludeIssuePRAuthors {
			for _, pr := range getLinkedPRs(allMergedPRs, issue) {
				if config.IncludeIssuePRs {
					references = append(references, change.Reference{
						Text: fmt.Sprintf("#%d", pr.Number),
						URL:  pr.URL,
					})
				}
				if config.IncludeIssuePRAuthors && pr.Author != "" {
					references = append(references, change.Reference{
						Text: fmt.Sprintf("@%s", pr.Author),
						URL:  fmt.Sprintf("https://%s/%s", config.Host, pr.Author),
					})
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
	return changes
}

func getLinkedPRs(allMergedPRs []ghPullRequest, issue ghIssue) (linked []ghPullRequest) {
	for _, pr := range allMergedPRs {
		for _, linkedIssue := range pr.LinkedIssues {
			if linkedIssue.URL == issue.URL {
				linked = append(linked, pr)
			}
		}
	}
	return linked
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

	filters = append(filters, standardChronologicalIssueFilters(sinceTag, untilTag)...)

	return filters
}

func standardChronologicalIssueFilters(sinceTag, untilTag *git.Tag) (filters []issueFilter) {
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
