package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
)

func Test_extractGithubUserAndRepo(t *testing.T) {
	tests := []struct {
		url  string
		user string
		repo string
	}{
		{
			url:  "git@github.com:someone/project.git",
			user: "someone",
			repo: "project",
		},
		{
			url:  "https://github.com/someone/project.git",
			user: "someone",
			repo: "project",
		},
		{
			url:  "http://github.com/someone/project.git",
			user: "someone",
			repo: "project",
		},
	}
	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			user, repo := extractGithubUserAndRepo(test.url)
			assert.Equal(t, test.user, user, "bad user")
			assert.Equal(t, test.repo, repo, "bad repo")
		})
	}
}

func Test_issueFilters(t *testing.T) {
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)
	timeBefore := timeStart.Add(-2 * time.Hour)
	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	bugAfterLastRelease := ghIssue{
		Title:    "bug after last release",
		Number:   1,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	bugAtLastRelease := ghIssue{
		Title:    "bug at (included within) last release",
		Number:   6,
		ClosedAt: timeStart,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	bugAtEndTag := ghIssue{
		Title:    "bug at (included within) end tag",
		Number:   7,
		ClosedAt: timeEnd,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueWithoutLabelAfterLastRelease := ghIssue{
		Title:    "issue without label after last release",
		Number:   2,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{},
	}

	bugBeforeLastRelease := ghIssue{
		Title:    "bug before last release",
		Number:   3,
		ClosedAt: timeBefore,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	bugAfterEndTag := ghIssue{
		Title:    "bug after end tag",
		Number:   4,
		ClosedAt: timeAfterEnd,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	featureAfterLastRelease := ghIssue{
		Title:    "feature after last release",
		Number:   5,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"feature"},
	}

	input := []ghIssue{
		// keep
		bugAfterLastRelease,
		issueWithoutLabelAfterLastRelease,
		featureAfterLastRelease,
		bugAtEndTag, // edge case
		// not keep
		bugBeforeLastRelease,
		bugAfterEndTag,
		bugAtLastRelease, // edge case
	}
	tests := []struct {
		name           string
		since          *git.Tag
		until          *git.Tag
		config         Config
		inputIssues    []ghIssue
		expectedIssues []ghIssue
	}{
		{
			name:  "keep changes between the tags",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputIssues: input,
			expectedIssues: []ghIssue{
				bugAfterLastRelease,
				featureAfterLastRelease,
				bugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      nil,
				ChangeTypesByLabel: changeTypeSet,
			},
			inputIssues: input,
			expectedIssues: []ghIssue{
				bugAfterLastRelease,
				bugAfterEndTag,
				featureAfterLastRelease,
				bugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag (that are not bugs)",
			since: sinceTag,
			config: Config{
				ExcludeLabels:      []string{"bug"},
				ChangeTypesByLabel: changeTypeSet,
			},
			inputIssues: input,
			expectedIssues: []ghIssue{
				featureAfterLastRelease,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expectedIssues, filterIssues(tt.inputIssues, standardIssueFilters(tt.config, tt.since, tt.until)...))
		})
	}
}

func Test_prFilters(t *testing.T) {
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)
	timeBefore := timeStart.Add(-2 * time.Hour)
	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	mergeCommit1 := "commit-pr-1-hash"
	mergeCommit2 := "commit-pr-2-hash"
	mergeCommit3 := "commit-pr-3-hash"
	// note: 4-5 reserved for issues... (not PRs)
	mergeCommit6 := "commit-pr-6-hash"
	mergeCommit7 := "commit-pr-7-hash"
	mergeCommit8 := "commit-pr-8-hash"
	mergeCommit9 := "commit-pr-9-hash"
	mergeCommit10 := "commit-pr-10-hash"
	mergeCommit11 := "commit-pr-11-hash"
	mergeCommit12 := "commit-pr-12-hash"
	// 13 is made up
	mergeCommit14 := "commit-pr-14-hash"

	tagRangeMergeCommits := []string{
		mergeCommit1,
		mergeCommit2,
		mergeCommit3,
		mergeCommit9,
		mergeCommit10,
		mergeCommit11,
		// note: merge commit 13 represents a PR that was merged after the end tag, thus us excluded via commit filter
	}

	outsideRangeMergeCommits := []string{
		mergeCommit6,
		mergeCommit7,
		mergeCommit8,
		mergeCommit12,
		mergeCommit14,
	}

	var allMergeCommits []string
	allMergeCommits = append(allMergeCommits, tagRangeMergeCommits...)
	allMergeCommits = append(allMergeCommits, outsideRangeMergeCommits...)

	prBugAfterLastRelease := ghPullRequest{
		Title:       "pr bug after starting tag",
		Number:      1,
		MergedAt:    timeAfter,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit1,
	}

	prBugAtLastRelease := ghPullRequest{
		Title:       "pr bug at (within) last release",
		Number:      10,
		MergedAt:    timeStart,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit10,
	}

	prBugAtEndTag := ghPullRequest{
		Title:       "pr bug at (within) end tag",
		Number:      11,
		MergedAt:    timeEnd,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit3,
	}

	prAfterLastRelease := ghPullRequest{
		Title:       "pr after starting tag",
		Number:      9,
		MergedAt:    timeAfter,
		MergeCommit: mergeCommit9,
	}

	prBugBeforeLastRelease := ghPullRequest{
		Title:       "pr bug before starting tag",
		Number:      2,
		MergedAt:    timeBefore,
		Labels:      []string{"bug"},
		MergeCommit: mergeCommit2,
	}

	issueClosedAfterLastRelease := ghIssue{
		Title:    "issue bug (closed after last release)",
		Number:   4,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueOpen := ghIssue{
		Title:  "issue bug (open)",
		Number: 5,
		Closed: false,
		Labels: []string{"bug"},
	}

	prBugAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ open linked issue)",
		Number:       14,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueOpen},
		MergeCommit:  mergeCommit14,
	}

	prAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ open linked issue)",
		Number:       6,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueOpen},
		MergeCommit:  mergeCommit6,
	}

	prBugAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ closed linked issue)",
		Number:       7,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
		MergeCommit:  mergeCommit7,
	}

	prAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ closed linked issue)",
		Number:       8,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
		MergeCommit:  mergeCommit8,
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:       "pr feature after end tag",
		Number:      12,
		MergedAt:    timeAfterEnd,
		Labels:      []string{"feature"},
		MergeCommit: mergeCommit12,
	}

	prFeatureAfterEndTagAndMergeRange := ghPullRequest{
		Title:       "pr feature after end tag (and merge range)",
		Number:      13,
		MergedAt:    timeAfterEnd,
		Labels:      []string{"feature"},
		MergeCommit: "made-up-commit-hash",
	}

	// no change-type label, but a conventional-commit title that resolves to a
	// change type only when title inference is enabled.
	prConventionalCommitNoLabel := ghPullRequest{
		Title:       "feat: pr with conventional-commit title and no label",
		Number:      15,
		MergedAt:    timeAfter,
		MergeCommit: mergeCommit1,
	}

	input := []ghPullRequest{
		// keep
		prBugAfterLastRelease,
		prBugAtEndTag, // edge case

		// filter out
		prAfterLastRelease,
		prBugAtLastRelease, // edge case
		prBugBeforeLastRelease,
		prBugAfterLastReleaseWithOpenLinkedIssue,
		prAfterLastReleaseWithOpenLinkedIssue,
		prBugAfterLastReleaseWithClosedLinkedIssue,
		prAfterLastReleaseWithClosedLinkedIssue,
		prFeatureAfterEndTag,
		prFeatureAfterEndTagAndMergeRange,
	}

	tests := []struct {
		name        string
		since       *git.Tag
		until       *git.Tag
		commits     []string
		config      Config
		inputPrs    []ghPullRequest
		expectedPrs []ghPullRequest
	}{
		{
			name:  "keep changes between the tags (dont consider PR commits)",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			commits:  tagRangeMergeCommits,
			expectedPrs: []ghPullRequest{
				prBugAfterLastRelease,
				prBugAtEndTag,
			},
		},
		{
			name:  "keep changes after start tag (dont consider PR commits)",
			since: sinceTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			commits:  allMergeCommits,
			expectedPrs: []ghPullRequest{
				prBugAfterLastRelease,
				prBugAtEndTag,
				prFeatureAfterEndTag,
				prFeatureAfterEndTagAndMergeRange,
			},
		},
		{
			name:  "keep only added features after start tag (dont consider PR commits)",
			since: sinceTag,
			config: Config{
				ExcludeLabels:          []string{"bug"},
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			commits:  allMergeCommits,
			expectedPrs: []ghPullRequest{
				prFeatureAfterEndTag,
				prFeatureAfterEndTagAndMergeRange,
			},
		},
		{
			name:  "inference off drops unlabeled conventional-commit PR",
			since: sinceTag,
			config: Config{
				ChangeTypesByLabel:                  changeTypeSet,
				ChangeTypesByConventionalCommitType: change.TypeSet{"feat": feature},
				InferChangeTypeFromTitle:            false,
				ConsiderPRMergeCommits:              false,
			},
			inputPrs:    []ghPullRequest{prBugAfterLastRelease, prConventionalCommitNoLabel},
			commits:     allMergeCommits,
			expectedPrs: []ghPullRequest{prBugAfterLastRelease},
		},
		{
			name:  "inference on keeps unlabeled conventional-commit PR",
			since: sinceTag,
			config: Config{
				ChangeTypesByLabel:                  changeTypeSet,
				ChangeTypesByConventionalCommitType: change.TypeSet{"feat": feature},
				InferChangeTypeFromTitle:            true,
				ConsiderPRMergeCommits:              false,
			},
			inputPrs:    []ghPullRequest{prBugAfterLastRelease, prConventionalCommitNoLabel},
			commits:     allMergeCommits,
			expectedPrs: []ghPullRequest{prBugAfterLastRelease, prConventionalCommitNoLabel},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep := applyStandardPRFilters(tt.inputPrs, tt.config, tt.since, tt.until, tt.commits)
			assert.ElementsMatch(t, tt.expectedPrs, keep)
		})
	}
}

func Test_prFilters_byCommits(t *testing.T) {
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)

	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	mergeCommit12 := "commit-pr-12-hash"

	validMergeCommits := []string{
		mergeCommit12,
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:       "pr feature after end tag",
		Number:      12,
		MergedAt:    timeAfterEnd, // note: this is after the end tag...
		Labels:      []string{"feature"},
		MergeCommit: mergeCommit12, // continue note: ... but this commit is within the merge range
	}

	prFeatureBeforeEndTagButNotWithinMergeRange := ghPullRequest{
		Title:       "pr feature not within merge range",
		Number:      13,
		MergedAt:    timeAfter, // note: this is during the tag range...
		Labels:      []string{"feature"},
		MergeCommit: "made-up-commit-hash", // continue note: ... but this commit is not within the merge range
	}

	input := []ghPullRequest{
		// keep
		prFeatureAfterEndTag, // re-included via commit range

		// filter out
		prFeatureBeforeEndTagButNotWithinMergeRange,
	}

	tests := []struct {
		name        string
		since       *git.Tag
		until       *git.Tag
		commits     []string
		config      Config
		inputPrs    []ghPullRequest
		expectedPrs []ghPullRequest
	}{
		{
			name:  "add PRs merged after end tag if they are in the commit range",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: true,
			},
			inputPrs: input,
			commits:  validMergeCommits,
			expectedPrs: []ghPullRequest{
				prFeatureAfterEndTag,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keep := applyStandardPRFilters(tt.inputPrs, tt.config, tt.since, tt.until, tt.commits)
			assert.ElementsMatch(t, tt.expectedPrs, keep)
		})
	}
}

func Test_changesFromIssuesExtractedFromPRs(t *testing.T) {
	// log.Log = logger.NewLogrusLogger(logger.LogrusConfig{
	//	EnableConsole: true,
	//	EnableFile:    false,
	//	Structured:    false,
	//	Level:         logrus.TraceLevel,
	// })
	patch := change.NewType("patch", change.SemVerPatch)
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-change", change.SemVerMajor)

	changeTypeSet := change.TypeSet{
		"bug":             patch,
		"fix":             patch,
		"feature":         feature,
		"breaking":        breaking,
		"removed":         breaking,
		"breaking-change": breaking,
	}

	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)
	timeAfter := timeStart.Add(2 * time.Hour)
	timeBefore := timeStart.Add(-2 * time.Hour)
	timeEnd := timeStart.Add(5 * time.Hour)
	timeAfterEnd := timeEnd.Add(3 * time.Hour)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart,
	}

	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: timeEnd,
	}

	prBugAfterLastRelease := ghPullRequest{
		Title:    "pr bug after starting tag",
		Number:   1,
		MergedAt: timeAfter,
		Labels:   []string{"bug"},
	}

	prBugAtLastRelease := ghPullRequest{
		Title:    "pr bug at (within) last release",
		Number:   10,
		MergedAt: timeStart,
		Labels:   []string{"bug"},
	}

	prBugAtEndTag := ghPullRequest{
		Title:    "pr bug at (within) end tag",
		Number:   11,
		MergedAt: timeEnd,
		Labels:   []string{"bug"},
	}

	prAfterLastRelease := ghPullRequest{
		Title:    "pr after starting tag",
		Number:   9,
		MergedAt: timeAfter,
	}

	prBugBeforeLastRelease := ghPullRequest{
		Title:    "pr bug before starting tag",
		Number:   2,
		MergedAt: timeBefore,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease := ghIssue{
		Title:    "issue bug (closed after last release)",
		Number:   4,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease2 := ghIssue{
		Title:    "issue bug (closed after last release) -- 2",
		Number:   13,
		ClosedAt: timeAfter,
		Closed:   true,
		Labels:   []string{"bug"},
	}

	issueClosedAfterLastRelease3 := ghIssue{
		Title:    "issue feature (closed after last release) -- 3",
		Number:   15,
		ClosedAt: timeAfterEnd,
		Closed:   true,
		Labels:   []string{"feature"},
	}

	issueOpen := ghIssue{
		Title:  "issue bug (open)",
		Number: 5,
		Closed: false,
		Labels: []string{"bug"},
	}

	prBugAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ open linked issue)",
		Number:       3,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueOpen},
	}

	prAfterLastReleaseWithOpenLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ open linked issue)",
		Number:       6,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueOpen},
	}

	prBugAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr bug after starting tag (w/ closed linked issue)",
		Number:       7,
		MergedAt:     timeAfter,
		Labels:       []string{"bug"},
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease},
	}

	prAfterLastReleaseWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after starting tag (w/ closed linked issue)",
		Number:       8,
		MergedAt:     timeAfter,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease2},
	}

	prFeatureAfterEndTag := ghPullRequest{
		Title:    "pr feature after end tag",
		Number:   12,
		MergedAt: timeAfterEnd,
		Labels:   []string{"feature"},
	}

	prAfterEndTagWithClosedLinkedIssue := ghPullRequest{
		Title:        "pr after end tag (w/ closed linked issue)",
		Number:       14,
		MergedAt:     timeAfterEnd,
		Labels:       nil,
		LinkedIssues: []ghIssue{issueClosedAfterLastRelease3},
	}

	input := []ghPullRequest{
		// keep
		prAfterLastReleaseWithClosedLinkedIssue, // = issue "issueClosedAfterLastRelease2"
		prAfterEndTagWithClosedLinkedIssue,      // = issue "issueClosedAfterLastRelease3"
		// filter out
		prAfterLastRelease,
		prBugAtLastRelease, // edge case
		prBugBeforeLastRelease,
		prBugAfterLastReleaseWithOpenLinkedIssue,
		prAfterLastReleaseWithOpenLinkedIssue,
		prBugAfterLastRelease,
		prBugAtEndTag, // edge case
		prAfterLastReleaseWithClosedLinkedIssue,
		prFeatureAfterEndTag,
		// why not this one? PRs with these labels should explicitly be used in the changelog directly (not the corresponding linked issue)
		prBugAfterLastReleaseWithClosedLinkedIssue, // = issue "issueClosedAfterLastRelease",
	}

	tests := []struct {
		name           string
		since          *git.Tag
		until          *git.Tag
		config         Config
		inputPrs       []ghPullRequest
		commits        []string
		expectedIssues []ghIssue
	}{
		{
			name:  "keep changes between the tags for only closed PRs with linked closed issues",
			since: sinceTag,
			until: untilTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease2,
			},
		},
		{
			name:  "keep changes after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:          nil,
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease2,
				issueClosedAfterLastRelease3,
			},
		},
		{
			name:  "keep only added features after start tag",
			since: sinceTag,
			config: Config{
				ExcludeLabels:          []string{"bug"},
				ChangeTypesByLabel:     changeTypeSet,
				ConsiderPRMergeCommits: false,
			},
			inputPrs: input,
			expectedIssues: []ghIssue{
				issueClosedAfterLastRelease3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expectedIssues, issuesExtractedFromPRs(tt.config, tt.inputPrs, tt.since, tt.until, tt.commits))
		})
	}
}

func Test_createChangesFromIssues(t *testing.T) {
	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)

	patch := change.NewType("patch", change.SemVerPatch)

	changeTypeSet := change.TypeSet{
		"bug": patch,
	}

	issue1 := ghIssue{
		Title:    "Issue 1",
		Number:   1,
		URL:      "issue-1-url",
		ClosedAt: timeStart,
		Labels:   []string{"bug"},
	}

	issue2 := ghIssue{
		Title:    "Issue 2",
		Number:   2,
		URL:      "issue-2-url",
		ClosedAt: timeStart,
		Labels:   []string{"bug"},
	}

	issue3 := ghIssue{
		Title:    "Issue 3 no PRs",
		Number:   3,
		URL:      "issue-3-url",
		ClosedAt: timeStart,
		Labels:   []string{"bug"},
	}

	prWithLinkedIssues1 := ghPullRequest{
		Title:    "pr 1 with linked issues",
		MergedAt: timeStart,
		Number:   1,
		Labels:   []string{"bug"},
		Author:   "some-author-1",
		URL:      "pr-1-url",
		LinkedIssues: []ghIssue{
			issue1,
		},
	}

	prWithLinkedIssues2 := ghPullRequest{
		Title:    "pr 2 with linked issues",
		MergedAt: timeStart,
		Number:   2,
		Labels:   []string{"another-label"},
		Author:   "some-author-2",
		URL:      "pr-2-url",
		LinkedIssues: []ghIssue{
			issue1,
			issue2,
		},
	}

	prWithoutLinkedIssues1 := ghPullRequest{
		MergedAt: timeStart,
		Title:    "pr 3 without linked issues",
		Number:   3,
		Author:   "some-author",
		URL:      "pr-3-url",
	}

	tests := []struct {
		name            string
		config          Config
		inputPrs        []ghPullRequest
		issues          []ghIssue
		expectedChanges []change.Change
	}{
		{
			name: "includes author for issues",
			config: Config{
				IncludeIssuePRAuthors: true,
				IncludeIssuePRs:       true,
				ChangeTypesByLabel:    changeTypeSet,
				Host:                  "some-host",
			},
			inputPrs: []ghPullRequest{
				prWithLinkedIssues1,
				prWithLinkedIssues2,
				prWithoutLinkedIssues1,
			},
			issues: []ghIssue{
				issue1,
				issue2,
				issue3,
			},
			expectedChanges: []change.Change{
				{
					Text:        "Issue 1",
					ChangeTypes: []change.Type{patch},
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#1",
							URL:  "issue-1-url",
						},
						{
							Text: "#1",
							URL:  "pr-1-url",
						},
						{
							Text: "@some-author-1",
							URL:  "https://some-host/some-author-1",
						},
						{
							Text: "#2",
							URL:  "pr-2-url",
						},
						{
							Text: "@some-author-2",
							URL:  "https://some-host/some-author-2",
						},
					},
					EntryType: "githubIssue",
					Entry:     issue1,
				},
				{
					Text:        "Issue 2",
					ChangeTypes: []change.Type{patch},
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#2",
							URL:  "issue-2-url",
						},
						{
							Text: "#2",
							URL:  "pr-2-url",
						},
						{
							Text: "@some-author-2",
							URL:  "https://some-host/some-author-2",
						},
					},
					EntryType: "githubIssue",
					Entry:     issue2,
				},
				{
					Text:        "Issue 3 no PRs",
					ChangeTypes: []change.Type{patch},
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#3",
							URL:  "issue-3-url",
						},
					},
					EntryType: "githubIssue",
					Entry:     issue3,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := createChangesFromIssues(tt.config, tt.inputPrs, tt.issues)
			if !reflect.DeepEqual(tt.expectedChanges, changes) {
				// print out a JSON diff
				assert.JSONEq(t, toJson(t, tt.expectedChanges), toJson(t, changes))
			}
		})
	}
}

func Test_changesFromUnlabeledPRs(t *testing.T) {
	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart.Add(-5 * time.Hour),
	}

	prWithLabels := ghPullRequest{
		Title:    "pr with labels",
		MergedAt: timeStart,
		Number:   3,
		Labels:   []string{"bug"},
		Author:   "nobody",
		URL:      "no-url",
	}

	prWithLabels2 := ghPullRequest{
		Title:    "pr with labels 2",
		MergedAt: timeStart,
		Number:   4,
		Labels:   []string{"another-label"},
		Author:   "nobody",
		URL:      "no-url",
	}

	prWithoutLabels := ghPullRequest{
		MergedAt: timeStart,
		Title:    "pr without labels",
		Number:   6,
		Author:   "some-author",
		URL:      "some-url",
	}

	prWithoutLabels2 := ghPullRequest{
		MergedAt: timeStart,
		Title:    "pr without labels 2",
		Number:   7,
		Author:   "some-author-2",
		URL:      "some-url-2",
	}

	tests := []struct {
		name            string
		config          Config
		inputPrs        []ghPullRequest
		expectedChanges []change.Change
	}{
		{
			name: "includes only unlabeled PRs",
			config: Config{
				Host: "some-host",
			},
			inputPrs: []ghPullRequest{
				// filter
				prWithLabels,
				prWithLabels2,
				// keep
				prWithoutLabels,
				prWithoutLabels2,
			},
			expectedChanges: []change.Change{
				{
					Text:        "pr without labels",
					ChangeTypes: change.UnknownTypes,
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#6",
							URL:  "some-url",
						},
						{
							Text: "@some-author",
							URL:  "https://some-host/some-author",
						},
					},
					EntryType: "githubPR",
					Entry:     prWithoutLabels,
				},
				{
					Text:        "pr without labels 2",
					ChangeTypes: change.UnknownTypes,
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#7",
							URL:  "some-url-2",
						},
						{
							Text: "@some-author-2",
							URL:  "https://some-host/some-author-2",
						},
					},
					EntryType: "githubPR",
					Entry:     prWithoutLabels2,
				},
			},
		},
		{
			// with title inference on, an unlabeled PR whose change type is inferred
			// from its conventional-commit title is carried by the standard (typed)
			// path, so it must be excluded here to avoid double-counting.
			name: "excludes unlabeled PR with inferable conventional-commit title",
			config: Config{
				Host:                     "some-host",
				InferChangeTypeFromTitle: true,
				ChangeTypesByConventionalCommitType: change.TypeSet{
					"feat": change.NewType("added-feature", change.SemVerMinor),
				},
			},
			inputPrs: []ghPullRequest{
				// filter: change type inferred from title
				{
					MergedAt: timeStart,
					Title:    "feat: shiny new thing",
					Number:   8,
					Author:   "cc-author",
					URL:      "cc-url",
				},
				// keep: not a conventional commit
				prWithoutLabels,
			},
			expectedChanges: []change.Change{
				{
					Text:        "pr without labels",
					ChangeTypes: change.UnknownTypes,
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#6",
							URL:  "some-url",
						},
						{
							Text: "@some-author",
							URL:  "https://some-host/some-author",
						},
					},
					EntryType: "githubPR",
					Entry:     prWithoutLabels,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := changesFromUnlabeledPRs(tt.config, tt.inputPrs, sinceTag, nil, nil)
			if !reflect.DeepEqual(tt.expectedChanges, changes) {
				// print out a JSON diff
				assert.JSONEq(t, toJson(t, tt.expectedChanges), toJson(t, changes))
			}
		})
	}
}

func Test_changesFromUnlabeledIssues(t *testing.T) {
	timeStart := time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC)

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: timeStart.Add(-5 * time.Hour),
	}

	issueWithLabels := ghIssue{
		Title:    "issue with labels",
		ClosedAt: timeStart,
		Number:   3,
		Labels:   []string{"bug"},
		Author:   "nobody",
		URL:      "no-url",
	}

	issueWithLabels2 := ghIssue{
		Title:    "issue with labels 2",
		ClosedAt: timeStart,
		Number:   4,
		Labels:   []string{"another-label"},
		Author:   "nobody",
		URL:      "no-url",
	}

	issueWithoutLabels := ghIssue{
		ClosedAt: timeStart,
		Title:    "issue without labels",
		Number:   6,
		Author:   "some-author",
		URL:      "some-url",
	}

	issueWithoutLabels2 := ghIssue{
		ClosedAt: timeStart,
		Title:    "issue without labels 2",
		Number:   7,
		Author:   "some-author-2",
		URL:      "some-url-2",
	}

	pr1 := ghPullRequest{
		MergedAt: timeStart,
		Title:    "pr 1",
		Number:   1,
		LinkedIssues: []ghIssue{
			issueWithoutLabels,
		},
		URL:    "pr-1-url",
		Author: "pr-1-author",
	}

	tests := []struct {
		name            string
		config          Config
		prs             []ghPullRequest
		issues          []ghIssue
		expectedChanges []change.Change
	}{
		{
			name: "includes only unlabeled issues",
			config: Config{
				ChangeTypesByLabel: change.TypeSet{
					"bug": change.NewType("bug", change.SemVerPatch),
				},
				IncludeIssuePRs:        true,
				IncludeIssuePRAuthors:  true,
				IncludeUnlabeledIssues: true,
				Host:                   "some-host",
			},
			prs: []ghPullRequest{
				pr1,
			},
			issues: []ghIssue{
				issueWithLabels,
				issueWithLabels2,
				issueWithoutLabels,
				issueWithoutLabels2,
			},
			expectedChanges: []change.Change{
				{
					Text:        "issue without labels",
					ChangeTypes: change.UnknownTypes,
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#6",
							URL:  "some-url",
						},
						{
							Text: "#1",
							URL:  "pr-1-url",
						},
						{
							Text: "@pr-1-author",
							URL:  "https://some-host/pr-1-author",
						},
					},
					EntryType: "githubIssue",
					Entry:     issueWithoutLabels,
				},
				{
					Text:        "issue without labels 2",
					ChangeTypes: change.UnknownTypes,
					Timestamp:   timeStart,
					References: []change.Reference{
						{
							Text: "#7",
							URL:  "some-url-2",
						},
					},
					EntryType: "githubIssue",
					Entry:     issueWithoutLabels2,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := changesFromUnlabeledIssues(tt.config, tt.prs, tt.issues, sinceTag, nil)
			if !reflect.DeepEqual(tt.expectedChanges, changes) {
				// print out a JSON diff
				assert.JSONEq(t, toJson(t, tt.expectedChanges), toJson(t, changes))
			}
		})
	}
}

func toJson(t *testing.T, changes []change.Change) string {
	out, err := json.Marshal(changes)
	require.NoError(t, err)
	return string(out)
}

func TestSummarizer_getChangeScope(t *testing.T) {
	testTime := time.Now()
	tests := []struct {
		name           string
		repoFixture    string
		config         Config
		sinceRef       string
		untilRef       string
		releaseFetcher releaseFetcher
		want           *changeScope
		wantErr        assert.ErrorAssertionFunc
	}{
		{
			name:        "tagged start and end state (go case for release)",
			repoFixture: "testdata/repos/v0.2.0-repo",
			sinceRef:    "v0.1.0", // the caller infers this and passes it explicitly (tag only)
			untilRef:    "v0.2.0", // the caller infers this and passes it explicitly (tag only)
			config: Config{
				ConsiderPRMergeCommits: true,
			},
			releaseFetcher: func(_, _, tag string) (*ghRelease, error) {
				assert.Equal(t, "v0.1.0", tag)
				return &ghRelease{
					Tag:      tag,
					Date:     testTime,
					IsLatest: true,
					IsDraft:  false,
				}, nil
			},
			want: &changeScope{
				Commits: gitLogRange(t, "testdata/repos/v0.2.0-repo", "v0.1.0", "v0.2.0", false),
				Start: changePoint{
					Ref: "v0.1.0",
					Tag: &git.Tag{
						Name:      "v0.1.0",
						Timestamp: testTime,
						Commit:    gitTagCommit(t, "testdata/repos/v0.2.0-repo", "v0.1.0"),
					},
					Inclusive: false,
					Timestamp: &testTime,
				},
				End: changePoint{
					Ref: "v0.2.0",
					Tag: &git.Tag{
						Name:      "v0.2.0",
						Timestamp: testTime,
						Commit:    gitTagCommit(t, "testdata/repos/v0.2.0-repo", "v0.2.0"),
					},
					Inclusive: true,
					Timestamp: &testTime,
				},
			},
		},
		{
			name:        "tagged start but not end state",
			repoFixture: "testdata/repos/v0.3.0-dev-repo",
			sinceRef:    "v0.2.0", // the caller infers this and passes it explicitly (tag only)
			untilRef:    "",
			config: Config{
				ConsiderPRMergeCommits: true,
			},
			releaseFetcher: func(_, _, tag string) (*ghRelease, error) {
				assert.Equal(t, "v0.2.0", tag)
				return &ghRelease{
					Tag:      tag,
					Date:     testTime,
					IsLatest: true,
					IsDraft:  false,
				}, nil
			},
			want: &changeScope{
				Commits: gitLogRange(t, "testdata/repos/v0.3.0-dev-repo", "v0.2.0", "", false),
				Start: changePoint{
					Ref: "v0.2.0",
					Tag: &git.Tag{
						Name:      "v0.2.0",
						Timestamp: testTime,
						Commit:    gitTagCommit(t, "testdata/repos/v0.3.0-dev-repo", "v0.2.0"),
					},
					Inclusive: false,
					Timestamp: &testTime,
				},
				End: changePoint{
					Ref:       gitHeadCommit(t, "testdata/repos/v0.3.0-dev-repo"),
					Tag:       nil,
					Inclusive: true,
					Timestamp: nil,
				},
			},
		},
		{
			name:        "first release (already has start tag)",
			repoFixture: "testdata/repos/v0.3.0-dev-repo",
			sinceRef:    "v0.2.0",
			untilRef:    "",
			config: Config{
				ConsiderPRMergeCommits: true,
			},
			releaseFetcher: func(_, _, _ string) (*ghRelease, error) {
				return nil, nil
			},
			want: &changeScope{
				Commits: gitLogRange(t, "testdata/repos/v0.3.0-dev-repo", "v0.2.0", "", false),
				Start: changePoint{
					Ref: "v0.2.0",
					Tag: &git.Tag{
						Name:      "v0.2.0",
						Timestamp: testTime,
						Commit:    gitTagCommit(t, "testdata/repos/v0.3.0-dev-repo", "v0.2.0"),
					},
					Inclusive: false,
					Timestamp: nil, // this is the difference between this test and the previous one
				},
				End: changePoint{
					Ref:       gitHeadCommit(t, "testdata/repos/v0.3.0-dev-repo"),
					Tag:       nil,
					Inclusive: true,
					Timestamp: nil,
				},
			},
		},
		{
			name:        "first release (no tag found)",
			repoFixture: "testdata/repos/v0.1.0-dev-repo",
			sinceRef:    "",
			untilRef:    "",
			config: Config{
				ConsiderPRMergeCommits: true,
			},
			releaseFetcher: func(_, _, _ string) (*ghRelease, error) {
				return nil, nil
			},
			want: &changeScope{
				Commits: gitLogRange(t, "testdata/repos/v0.1.0-dev-repo", "", "", false),
				Start: changePoint{
					Ref:       gitFirstCommit(t, "testdata/repos/v0.1.0-dev-repo"),
					Tag:       nil,
					Inclusive: true, // this is the difference between this test and others
					Timestamp: nil,
				},
				End: changePoint{
					Ref:       gitHeadCommit(t, "testdata/repos/v0.1.0-dev-repo"),
					Tag:       nil,
					Inclusive: true,
					Timestamp: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = assert.NoError
			}

			gitter, err := git.New(tt.repoFixture)
			require.NoError(t, err)

			// we want to use the real git interface, however, we do not want to put any timestamps
			// under test, so we have a mock to override the timestamp
			gitter = mockGitter{
				timestamp: testTime,
				Interface: gitter,
			}

			s, err := NewSummarizer(gitter, tt.config)
			require.NoError(t, err)
			s.releaseFetcher = tt.releaseFetcher

			got, err := s.getChangeScope(tt.sinceRef, tt.untilRef)
			if !tt.wantErr(t, err, fmt.Sprintf("getChangeScope(%v, %v)", tt.sinceRef, tt.untilRef)) {
				return
			}

			assert.Equalf(t, tt.want, got, "getChangeScope(%v, %v)", tt.sinceRef, tt.untilRef)
		})
	}
}

func TestSummarizer_scopeHasNoCommits(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		scope  changeScope
		want   bool
	}{
		{
			name:   "merge-commit mode with no commits short-circuits",
			config: Config{ConsiderPRMergeCommits: true},
			scope:  changeScope{Commits: nil},
			want:   true,
		},
		{
			name:   "merge-commit mode with commits does not short-circuit",
			config: Config{ConsiderPRMergeCommits: true},
			scope:  changeScope{Commits: []string{"abc123"}},
			want:   false,
		},
		{
			// timestamp-only mode never populates Commits, so an empty slice must
			// not be read as "no changes" — that would skip every changelog.
			name:   "timestamp-only mode never short-circuits",
			config: Config{ConsiderPRMergeCommits: false},
			scope:  changeScope{Commits: nil},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Summarizer{config: tt.config}
			require.Equal(t, tt.want, s.scopeHasNoCommits(tt.scope))
		})
	}
}

func TestSummarizer_Changes_noCommitsShortCircuit(t *testing.T) {
	testTime := time.Now()

	gitter, err := git.New("testdata/repos/v0.2.0-repo")
	require.NoError(t, err)
	gitter = mockGitter{timestamp: testTime, Interface: gitter}

	s, err := NewSummarizer(gitter, Config{ConsiderPRMergeCommits: true})
	require.NoError(t, err)
	s.releaseFetcher = func(_, _, tag string) (*ghRelease, error) {
		return &ghRelease{Tag: tag, Date: testTime, IsLatest: true}, nil
	}

	// since == until: HEAD sits exactly on the release tag, so the range holds
	// no commits. The short-circuit must return an empty changelog without
	// reaching the GitHub API (this test runs without network access; a missed
	// short-circuit would attempt a fetch and fail).
	changes, err := s.Changes("v0.2.0", "v0.2.0")
	require.NoError(t, err)
	require.Empty(t, changes)

	prs, issues, commits := s.EvidenceTotals()
	require.Zero(t, prs)
	require.Zero(t, issues)
	require.Zero(t, commits)
}

func TestSummarizer_Trunk_noCommitsShortCircuit(t *testing.T) {
	testTime := time.Now()

	gitter, err := git.New("testdata/repos/v0.2.0-repo")
	require.NoError(t, err)
	gitter = mockGitter{timestamp: testTime, Interface: gitter}

	s, err := NewSummarizer(gitter, Config{ConsiderPRMergeCommits: true})
	require.NoError(t, err)
	s.releaseFetcher = func(_, _, tag string) (*ghRelease, error) {
		return &ghRelease{Tag: tag, Date: testTime, IsLatest: true}, nil
	}

	// same since==until range as above: an empty commit set must yield an empty
	// trunk view without any GitHub API fetch.
	data, err := s.Trunk("v0.2.0", "v0.2.0")
	require.NoError(t, err)
	require.NotNil(t, data)
	require.Empty(t, data.Commits)
}

type mockGitter struct {
	timestamp time.Time
	git.Interface
}

func (m mockGitter) SearchForTag(tagRef string) (*git.Tag, error) {
	a, err := m.Interface.SearchForTag(tagRef)
	if a != nil {
		a.Timestamp = m.timestamp
	}
	return a, err
}

func gitFirstCommit(t *testing.T, path string) string {
	t.Helper()

	cmd := exec.Command("git", "--no-pager", "log", "--reverse", `--pretty=format:%H`)
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.NotEmpty(t, rows)
	return rows[0]
}

func TestSummarizer_ChangesURL(t *testing.T) {
	tests := []struct {
		name     string
		sinceRef string
		untilRef string
		want     string
	}{
		{
			name:     "normal compare URL",
			sinceRef: "v0.1.0",
			untilRef: "v0.2.0",
			want:     "https://github.com/owner/repo/compare/v0.1.0...v0.2.0",
		},
		{
			name:     "empty sinceRef returns commits URL",
			sinceRef: "",
			untilRef: "v0.2.0",
			want:     "https://github.com/owner/repo/commits/v0.2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Summarizer{
				userName: "owner",
				repoName: "repo",
				config: Config{
					Host: "github.com",
				},
			}

			got := s.ChangesURL(tt.sinceRef, tt.untilRef)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_prChangeTypes(t *testing.T) {
	feature := change.NewType("added-feature", change.SemVerMinor)
	bugFix := change.NewType("bug-fix", change.SemVerPatch)
	breaking := change.NewType("breaking-feature", change.SemVerMajor)

	labelSet := change.TypeSet{
		"enhancement": feature,
		"bug":         bugFix,
	}
	prefixSet := change.TypeSet{
		"feat":                      feature,
		"fix":                       bugFix,
		change.BreakingChangePrefix: breaking,
	}

	config := Config{
		InferChangeTypeFromTitle:            true,
		ChangeTypesByLabel:                  labelSet,
		ChangeTypesByConventionalCommitType: prefixSet,
	}

	tests := []struct {
		name   string
		config Config
		pr     ghPullRequest
		want   []change.Type
	}{
		{
			name:   "label resolves change type",
			config: config,
			pr:     ghPullRequest{Title: "anything", Labels: []string{"bug"}},
			want:   []change.Type{bugFix},
		},
		{
			name:   "label wins over conventional-commit title",
			config: config,
			pr:     ghPullRequest{Title: "feat: add a thing", Labels: []string{"bug"}},
			want:   []change.Type{bugFix},
		},
		{
			name:   "infer from feat title when unlabeled",
			config: config,
			pr:     ghPullRequest{Title: "feat: add a thing"},
			want:   []change.Type{feature},
		},
		{
			name:   "infer from fix title when label is not a change-type label",
			config: config,
			pr:     ghPullRequest{Title: "fix: squash a bug", Labels: []string{"size/L"}},
			want:   []change.Type{bugFix},
		},
		{
			name:   "breaking marker takes precedence over base type",
			config: config,
			pr:     ghPullRequest{Title: "feat!: drop legacy API"},
			want:   []change.Type{breaking},
		},
		{
			// when "!" is not mapped, a breaking PR falls back to its base type.
			name: "breaking marker falls back to base type when unmapped",
			config: Config{
				InferChangeTypeFromTitle:            true,
				ChangeTypesByLabel:                  labelSet,
				ChangeTypesByConventionalCommitType: change.TypeSet{"feat": feature},
			},
			pr:   ghPullRequest{Title: "feat!: drop legacy API"},
			want: []change.Type{feature},
		},
		{
			// the parser lowercases the type, so an uppercase title still resolves
			// against the lowercase-keyed prefix set.
			name:   "inference is case-insensitive on the title type",
			config: config,
			pr:     ghPullRequest{Title: "Feat: add a thing"},
			want:   []change.Type{feature},
		},
		{
			name:   "unmapped conventional-commit type yields nothing",
			config: config,
			pr:     ghPullRequest{Title: "docs: update the readme"},
			want:   nil,
		},
		{
			name:   "non-conventional title yields nothing",
			config: config,
			pr:     ghPullRequest{Title: "Bump foo to v2"},
			want:   nil,
		},
		{
			name: "inference disabled falls back to labels only",
			config: Config{
				InferChangeTypeFromTitle:            false,
				ChangeTypesByLabel:                  labelSet,
				ChangeTypesByConventionalCommitType: prefixSet,
			},
			pr:   ghPullRequest{Title: "feat: add a thing"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prChangeTypes(tt.config, tt.pr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_prHasUnmappedBreakingMarker(t *testing.T) {
	feature := change.NewType("added-feature", change.SemVerMinor)
	breaking := change.NewType("breaking-feature", change.SemVerMajor)

	withBreaking := Config{
		InferChangeTypeFromTitle: true,
		ChangeTypesByConventionalCommitType: change.TypeSet{
			"feat":                      feature,
			change.BreakingChangePrefix: breaking,
		},
	}
	withoutBreaking := Config{
		InferChangeTypeFromTitle:            true,
		ChangeTypesByLabel:                  change.TypeSet{"bug": change.NewType("bug-fix", change.SemVerPatch)},
		ChangeTypesByConventionalCommitType: change.TypeSet{"feat": feature},
	}

	tests := []struct {
		name   string
		config Config
		pr     ghPullRequest
		want   bool
	}{
		{
			name:   "breaking marker with no breaking mapping warns (base type mapped)",
			config: withoutBreaking,
			pr:     ghPullRequest{Title: "feat!: drop legacy API"},
			want:   true,
		},
		{
			name:   "breaking marker with no breaking mapping warns (base type also unmapped)",
			config: withoutBreaking,
			pr:     ghPullRequest{Title: "chore!: drop legacy API"},
			want:   true,
		},
		{
			name:   "breaking marker mapped does not warn",
			config: withBreaking,
			pr:     ghPullRequest{Title: "feat!: drop legacy API"},
			want:   false,
		},
		{
			// an explicit change-type label short-circuits title inference, so the
			// title (and its breaking marker) is never consulted — warning would be
			// misleading.
			name:   "change-type label short-circuits inference, no warning",
			config: withoutBreaking,
			pr:     ghPullRequest{Title: "feat!: drop legacy API", Labels: []string{"bug"}},
			want:   false,
		},
		{
			name:   "non-breaking conventional commit does not warn",
			config: withoutBreaking,
			pr:     ghPullRequest{Title: "feat: add a thing"},
			want:   false,
		},
		{
			name:   "non-conventional title does not warn",
			config: withoutBreaking,
			pr:     ghPullRequest{Title: "Bump foo to v2"},
			want:   false,
		},
		{
			name: "inference disabled does not warn",
			config: Config{
				InferChangeTypeFromTitle:            false,
				ChangeTypesByConventionalCommitType: change.TypeSet{"feat": feature},
			},
			pr:   ghPullRequest{Title: "feat!: drop legacy API"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, prHasUnmappedBreakingMarker(tt.config, tt.pr))
		})
	}
}

func gitLogRange(t *testing.T, path, since, until string, startInclusive bool) []string {
	t.Helper()

	since = strings.TrimSpace(since)

	var modifier string
	if startInclusive {
		// why the ~1? we want git log to return inclusive results
		modifier = "~1"
	}

	args := []string{
		"--no-pager", "log", `--pretty=format:%H`,
	}

	if since != "" {
		args = append(args, fmt.Sprintf("%s%s..%s", since, modifier, until))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.NotEmpty(t, rows)
	return rows
}

func gitTagCommit(t *testing.T, path, tag string) string {
	t.Helper()

	tag = strings.TrimSpace(tag)
	if tag == "" {
		t.Fatal("require 'tag'")
	}

	// why the ~1? we want git log to return inclusive results
	cmd := exec.Command("git", "--no-pager", "log", `--pretty=format:%H`, fmt.Sprintf("%s~1..%s", tag, tag))
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 1 {
		t.Fatalf("unable to get commit for tag=%s: %q", tag, output)
	}
	require.NotEmpty(t, rows[0])
	return rows[0]
}

func gitHeadCommit(t *testing.T, path string) string {
	t.Helper()

	cmd := exec.Command("git", "--no-pager", "rev-parse", "HEAD")
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(rows) != 1 {
		t.Fatalf("unable to get commit for head: %q", output)
	}
	require.NotEmpty(t, rows[0])
	return rows[0]
}
