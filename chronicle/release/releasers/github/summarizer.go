package github

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anchore/chronicle/chronicle/event"
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

	// releaseCache memoizes per-tag release lookups so that the start-release
	// metadata fetched for the changelog header is not re-fetched again when
	// computing the change scope. Map presence (rather than non-nil value) is
	// the signal that a tag has been queried, so "no release for this tag"
	// results are cached too.
	releaseCache map[string]*ghRelease

	// raw evidence totals captured during the most recent Changes() call; read
	// by the worker for the summary report. Concurrent access is gated by mu,
	// which also guards the leaf fields below.
	mu          sync.Mutex
	prTotal     int
	issueTotal  int
	commitTotal int

	// kept-union counts captured at the end of the most recent Changes()
	// call: PRs (direct + indirect via linked issues), issues (direct), and
	// merge commits intersected with scope.Commits.
	prsKept           int
	issuesKept        int
	associatedCommits int

	// optional UI leaves plumbed in by SetEvidenceLeaves. P3 stores them for
	// P4 to use; this code does not yet update them.
	commitsLeaf *event.Leaf
	issuesLeaf  *event.Leaf
	prsLeaf     *event.Leaf
}

// SetEvidenceLeaves attaches UI evidence leaves to the summarizer. Any of the
// arguments may be nil. P4 will use these to publish live page-fetch progress;
// for now they are stored without being touched.
func (s *Summarizer) SetEvidenceLeaves(commits, issues, prs *event.Leaf) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commitsLeaf = commits
	s.issuesLeaf = issues
	s.prsLeaf = prs
}

// Repo returns the GitHub owner/repo this summarizer is targeting, derived
// from the git remote URL at construction time.
func (s *Summarizer) Repo() (user, repo string) {
	if s == nil {
		return "", ""
	}
	return s.userName, s.repoName
}

// EvidenceTotals returns the raw fetched counts captured during the most
// recent Changes() call: total PRs, total issues, total commits in scope.
func (s *Summarizer) EvidenceTotals() (prs, issues, commits int) {
	if s == nil {
		return 0, 0, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prTotal, s.issueTotal, s.commitTotal
}

// PRsKept returns the number of PRs that contributed to the changelog —
// directly (a PR-typed change) or indirectly via a linked issue that itself
// was kept. Captured at the end of the most recent Changes() call.
func (s *Summarizer) PRsKept() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.prsKept
}

// IssuesKept returns the number of issues directly kept in the changelog.
func (s *Summarizer) IssuesKept() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issuesKept
}

// AssociatedCommits returns the number of in-window merge commits whose PR
// (direct or indirectly via a linked issue) ended up in the changelog.
func (s *Summarizer) AssociatedCommits() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.associatedCommits
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
		return nil, fmt.Errorf("could not extract GitHub owner/repo from remote URL %q (expected formats: git@github.com:owner/repo.git or https://github.com/owner/repo.git)", repoURL)
	}

	log.WithFields("owner", user, "repo", repo).Info("🎯 targeting GitHub repository")

	if os.Getenv("GITHUB_TOKEN") == "" {
		log.Warn("GITHUB_TOKEN environment variable is not set; GitHub API requests will be unauthenticated and likely fail or be rate-limited (set a token with 'repo' scope, or 'public_repo' for public repositories)")
	} else {
		log.Info("GitHub API authentication: using GITHUB_TOKEN")
	}

	return &Summarizer{
		git:            gitter,
		userName:       user,
		repoName:       repo,
		config:         config,
		releaseFetcher: fetchRelease,
		releaseCache:   make(map[string]*ghRelease),
	}, nil
}

// fetchReleaseCached returns the release for the given tag, querying the API
// only on first lookup. Safe for single-goroutine use: callers are the
// changelog-info pre-flight and getSince, which both run on the main goroutine.
func (s *Summarizer) fetchReleaseCached(tag string) (*ghRelease, error) {
	if r, ok := s.releaseCache[tag]; ok {
		return r, nil
	}
	r, err := s.releaseFetcher(s.userName, s.repoName, tag)
	if err != nil {
		return nil, err
	}
	if s.releaseCache == nil {
		s.releaseCache = make(map[string]*ghRelease)
	}
	s.releaseCache[tag] = r
	return r, nil
}

func (s *Summarizer) Release(ref string) (*release.Release, error) {
	targetRelease, err := s.fetchReleaseCached(ref)
	if err != nil {
		return nil, err
	}
	if targetRelease == nil || targetRelease.Tag == "" {
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
	if sinceRef == "" {
		// no prior release, return commits page instead of invalid compare URL
		return fmt.Sprintf("https://%s/%s/%s/commits/%s", s.config.Host, s.userName, s.repoName, untilRef)
	}
	return fmt.Sprintf("https://%s/%s/%s/compare/%s...%s", s.config.Host, s.userName, s.repoName, sinceRef, untilRef)
}

func (s *Summarizer) LastRelease() (*release.Release, error) {
	latestRelease, err := fetchLatestNonDraftRelease(s.userName, s.repoName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch releases for %s/%s: %w", s.userName, s.repoName, err)
	}
	if latestRelease != nil {
		// seed the cache so that getSince doesn't re-fetch this same release
		// when it later needs the date for the timestamp filter
		if s.releaseCache == nil {
			s.releaseCache = make(map[string]*ghRelease)
		}
		s.releaseCache[latestRelease.Tag] = latestRelease
		return &release.Release{
			Version: latestRelease.Tag,
			Date:    latestRelease.Date,
		}, nil
	}
	// no releases found, return nil to signal "since the beginning"
	return nil, nil
}

func (s *Summarizer) Changes(sinceRef, untilRef string) ([]change.Change, error) {
	// surface commit-walk activity to the UI: the underlying git operation is
	// a single (fast) call so we can't tick per-commit, but at least flagging
	// "walking history" gives the leaf row a non-empty stage during work.
	s.mu.Lock()
	commitsLeaf := s.commitsLeaf
	s.mu.Unlock()
	commitsLeaf.SetStage("walking history")

	scope, err := s.getChangeScope(sinceRef, untilRef)
	if err != nil {
		return nil, err
	}

	if scope == nil {
		return nil, errors.New("could not determine start/end of change range (no scope produced)")
	}

	// scope is known; surface the commit count so the row stays informative
	// while the parallel PR/issue fetches in changes() run.
	commitsLeaf.SetStage(fmt.Sprintf("%d in scope", len(scope.Commits)))

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
			return nil, fmt.Errorf("unable to fetch commit range %q..%q: %w", sinceRef, untilRef, err)
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
		sinceRelease, err := s.fetchReleaseCached(sinceTag.Name)
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

	// capture commit total + UI leaves up front so the worker has a value to
	// report even if a later fetch fails, and so the leaves are read once
	// under the lock rather than repeatedly inside the goroutines below.
	s.mu.Lock()
	s.commitTotal = len(scope.Commits)
	prsLeaf := s.prsLeaf
	issuesLeaf := s.issuesLeaf
	s.mu.Unlock()

	// the merged-PR and closed-issue queries are independent paginated GraphQL
	// calls — they dominate runtime, so run them concurrently and join after.
	var (
		allMergedPRs    []ghPullRequest
		allClosedIssues []ghIssue
		prErr, issueErr error
		wg              sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		allMergedPRs, prErr = fetchMergedPRs(s.userName, s.repoName, scope.Start.Timestamp, prsLeaf)
	}()
	go func() {
		defer wg.Done()
		allClosedIssues, issueErr = fetchClosedIssues(s.userName, s.repoName, scope.Start.Timestamp, issuesLeaf)
	}()
	wg.Wait()

	if prErr != nil {
		return nil, prErr
	}
	if issueErr != nil {
		return nil, issueErr
	}

	s.mu.Lock()
	s.prTotal = len(allMergedPRs)
	s.issueTotal = len(allClosedIssues)
	s.mu.Unlock()

	log.WithFields("count", len(allMergedPRs), "since", scope.Start.Timestamp).Info("merged PRs discovered")

	if s.config.IncludePRs {
		changes = append(changes, changesFromStandardPRFilters(s.config, allMergedPRs, scope.Start.Tag, scope.End.Tag, scope.Commits)...)
	}

	if !s.config.IncludeIssuesClosedAsNotPlanned {
		allClosedIssues = filterIssues(allClosedIssues, excludeIssuesNotPlanned(allMergedPRs))
	}

	log.WithFields("count", len(allClosedIssues), "since", scope.Start.Timestamp).Info("closed issues discovered")

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

	s.captureEvidenceUnion(changes, allMergedPRs, scope.Commits)

	return changes, nil
}

// captureEvidenceUnion walks the assembled changes once and records the set
// of PRs and merge commits that contributed — directly (PR-typed change) OR
// indirectly via a kept linked-issue. The resulting counts are used by the
// post-teardown summary's evidence section. The mutex guards the same fields
// the worker reads via PRsKept / IssuesKept / AssociatedCommits accessors.
func (s *Summarizer) captureEvidenceUnion(changes []change.Change, allMergedPRs []ghPullRequest, scopeCommits []string) {
	scope := make(map[string]struct{}, len(scopeCommits))
	for _, sha := range scopeCommits {
		scope[sha] = struct{}{}
	}

	keptPRs := map[int]struct{}{}
	keptIssues := map[int]struct{}{}
	keptMergeCommits := map[string]struct{}{}

	addPR := func(pr ghPullRequest) {
		keptPRs[pr.Number] = struct{}{}
		if pr.MergeCommit != "" {
			keptMergeCommits[pr.MergeCommit] = struct{}{}
		}
	}

	for _, c := range changes {
		switch c.EntryType {
		case "githubPR":
			if pr, ok := c.Entry.(ghPullRequest); ok {
				addPR(pr)
			}
		case "githubIssue":
			is, ok := c.Entry.(ghIssue)
			if !ok {
				continue
			}
			keptIssues[is.Number] = struct{}{}
			// fold in PRs that linked this issue — they "indirectly contributed"
			// even if they aren't carried as their own change row.
			for _, lp := range getLinkedPRs(allMergedPRs, is) {
				addPR(lp)
			}
		}
	}

	associated := 0
	for sha := range keptMergeCommits {
		if _, ok := scope[sha]; ok {
			associated++
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.prsKept = len(keptPRs)
	s.issuesKept = len(keptIssues)
	s.associatedCommits = associated
}

func logChangeScope(c changeScope, considerCommits bool) {
	log.WithFields("since", c.Start.Ref, "until", c.End.Ref).Info("searching for changes")
	log.WithFields(changePointFields(c.Start)).Info("  ├── since")
	log.WithFields(changePointFields(c.End)).Info("  └── until")

	if considerCommits {
		log.WithFields("count", len(c.Commits)).Info("release comprises commits")
		logCommits(c.Commits)
	}

	// in a release process there tends to be a start point that is a github release and an end point that is a git tag.
	// in cases where the git tag is a lightweight tag encourage users to migrate to using annotated tags since
	// the annotated tag will have a timestamp associated with when the tag action was done and not when the PR merge
	// to main was done.
	// From https://git-scm.com/docs/git-tag:
	// > Annotated tags are meant for release while lightweight tags are meant for private or temporary object labels.
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
		// PRs with these labels should explicitly be used in the changelog directly (not the corresponding linked issue)
		prsWithoutLabel(config.ChangeTypesByLabel.Names()...),
		prsWithClosedLinkedIssue(),
	}

	if sinceTag != nil {
		prFilters = append([]prFilter{prsAfter(sinceTag.Timestamp.UTC())}, prFilters...)
	}

	if untilTag != nil {
		prFilters = append(prFilters, prsAtOrBefore(untilTag.Timestamp.UTC()))
	}

	includedPRs := applyPRFilters(allMergedPRs, config, sinceTag, untilTag, includeCommits, prFilters...)
	extractedIssues := uniqueIssuesFromPRs(includedPRs)

	// this represents the traits we wish to filter down to (not out).
	issueFilters := []issueFilter{
		issuesWithLabel(config.ChangeTypesByLabel.Names()...),
		issuesWithoutLabel(config.ExcludeLabels...),
	}

	if sinceTag != nil {
		issueFilters = append([]issueFilter{issuesAfter(sinceTag.Timestamp)}, issueFilters...)
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

	beforeChangeTypeFilter := len(includedPRs)
	var droppedNoChangeType []droppedPR
	includedPRs, droppedNoChangeType = filterPRs(includedPRs, prsWithChangeTypes(config))
	log.WithFields("kept", len(includedPRs), "dropped", len(droppedNoChangeType), "input", beforeChangeTypeFilter).Trace("PR change-type filter")

	log.WithFields("count", len(includedPRs)).Info("PRs contributing to changelog")
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
		log.Debugf("  %s #%d: merged %s", branch, pr.Number, internal.FormatDateTime(pr.MergedAt))
	}
}

func changesFromIssuesLinkedToPrs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag, includeCommits []string) []change.Change {
	// extract closed linked issues with closed PRs from the PR list. Why do this here?
	// githubs ontology has PRs as the source of truth for issue linking. Linked PR information
	// is not available on the issue itself.
	issues := issuesExtractedFromPRs(config, allMergedPRs, sinceTag, untilTag, includeCommits)
	issues = filterIssues(issues, issuesWithChangeTypes(config))

	log.WithFields("count", len(issues)).Info("linked issues contributing to changelog")
	logIssues(issues)

	return createChangesFromIssues(config, allMergedPRs, issues)
}

func changesFromIssues(config Config, allMergedPRs []ghPullRequest, allClosedIssues []ghIssue, sinceTag, untilTag *git.Tag) []change.Change {
	filteredIssues := filterIssues(allClosedIssues, standardIssueFilters(config, sinceTag, untilTag)...)

	filteredIssues = filterIssues(filteredIssues, issuesWithChangeTypes(config))

	log.WithFields("count", len(filteredIssues)).Info("issues contributing to changelog")
	logIssues(filteredIssues)

	return createChangesFromIssues(config, allMergedPRs, filteredIssues)
}

func logIssues(issues []ghIssue) {
	for idx, issue := range issues {
		var branch = treeBranch
		if idx == len(issues)-1 {
			branch = treeLeaf
		}
		log.Debugf("  %s #%d: closed %s", branch, issue.Number, internal.FormatDateTime(issue.ClosedAt))
	}
}

func changesFromUnlabeledPRs(config Config, allMergedPRs []ghPullRequest, sinceTag, untilTag *git.Tag, includeCommits []string) []change.Change {
	// this represents the traits we wish to filter down to (not out).
	filters := []prFilter{
		prsWithoutLabels(),
		prsWithoutLinkedIssues(),
	}

	filters = append(filters, standardChronologicalPrFilters(config, sinceTag, untilTag, includeCommits)...)

	filteredPRs, _ := filterPRs(allMergedPRs, filters...)

	log.WithFields("count", len(filteredPRs)).Info("unlabeled PRs contributing to changelog")
	logPRs(filteredPRs)

	return createChangesFromPRs(config, filteredPRs)
}

func changesFromUnlabeledIssues(config Config, allMergedPRs []ghPullRequest, allIssues []ghIssue, sinceTag, untilTag *git.Tag) []change.Change {
	// this represents the traits we wish to filter down to (not out).
	filters := standardChronologicalIssueFilters(sinceTag, untilTag)

	filters = append(filters, issuesWithoutLabels())

	filteredIssues := filterIssues(allIssues, filters...)

	log.WithFields("count", len(filteredIssues)).Info("unlabeled issues contributing to changelog")
	logIssues(filteredIssues)

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
