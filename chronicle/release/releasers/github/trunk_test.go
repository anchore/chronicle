package github

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/scylladb/go-set/strset"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
)

// shared test fixtures for trunk tests
var (
	trunkPatch   = change.NewType("patch", change.SemVerPatch)
	trunkFeature = change.NewType("added-feature", change.SemVerMinor)

	trunkChangeTypes = change.TypeSet{
		"bug":     trunkPatch,
		"feature": trunkFeature,
	}

	trunkBaseTime = time.Date(2024, time.January, 10, 12, 0, 0, 0, time.UTC)
)

// makeKeptByCommit is a helper to build the keptByCommit map from explicit
// commit→change pairs, avoiding repetitive map literal construction in tests.
func makeKeptByCommit(pairs ...interface{}) map[string][]change.Change {
	out := make(map[string][]change.Change)
	for i := 0; i+1 < len(pairs); i += 2 {
		hash := pairs[i].(string)
		c := pairs[i+1].(change.Change)
		out[hash] = append(out[hash], c)
	}
	return out
}

// prChange builds a change.Change whose Entry is a ghPullRequest.
func prChange(pr ghPullRequest, types ...change.Type) change.Change {
	return change.Change{
		ChangeTypes: types,
		Entry:       pr,
	}
}

// issueChange builds a change.Change whose Entry is a ghIssue.
func issueChange(issue ghIssue, types ...change.Type) change.Change {
	return change.Change{
		ChangeTypes: types,
		Entry:       issue,
	}
}

// sortChangeTypes is a cmp option for order-independent ChangeTypes comparison.
var sortChangeTypes = cmpopts.SortSlices(func(a, b change.Type) bool {
	return a.Name < b.Name
})

// sortTrunkIssues is a cmp option for order-independent TrunkIssue slice comparison.
var sortTrunkIssues = cmpopts.SortSlices(func(a, b release.TrunkIssue) bool {
	return a.Number < b.Number
})

func Test_mapKeptChangesToCommits(t *testing.T) {
	prHash := "aaaa1111"
	issueHash := "bbbb2222"

	pr := ghPullRequest{
		Number:      10,
		Title:       "fix bug via PR",
		MergeCommit: prHash,
		URL:         "https://github.com/owner/repo/pull/10",
	}

	issue := ghIssue{
		Number: 20,
		Title:  "fix bug via issue",
		URL:    "https://github.com/owner/repo/issues/20",
		Closed: true,
	}

	// a PR whose linked issues include our tracked issue
	closingPR := ghPullRequest{
		Number:      30,
		Title:       "fix bug (closes issue 20)",
		MergeCommit: issueHash,
		LinkedIssues: []ghIssue{
			{URL: issue.URL},
		},
	}

	// a second PR for the multi-change-same-commit case
	prHash2 := "cccc3333"
	pr2 := ghPullRequest{
		Number:      40,
		Title:       "another fix via PR",
		MergeCommit: prHash2,
	}

	tests := []struct {
		name          string
		keptChanges   []change.Change
		allMergedPRs  []ghPullRequest
		wantCommitMap map[string]int // commit hash → expected number of changes
	}{
		{
			name:         "PR-change maps via its own MergeCommit",
			keptChanges:  []change.Change{prChange(pr, trunkPatch)},
			allMergedPRs: []ghPullRequest{pr},
			wantCommitMap: map[string]int{
				prHash: 1,
			},
		},
		{
			name:         "issue-change maps via the closing PR's MergeCommit",
			keptChanges:  []change.Change{issueChange(issue, trunkFeature)},
			allMergedPRs: []ghPullRequest{closingPR},
			wantCommitMap: map[string]int{
				issueHash: 1,
			},
		},
		{
			name:          "issue-change with no closing PR is omitted",
			keptChanges:   []change.Change{issueChange(issue, trunkFeature)},
			allMergedPRs:  []ghPullRequest{pr}, // pr does not link to issue
			wantCommitMap: map[string]int{},
		},
		{
			name: "multiple changes for same commit both appear",
			keptChanges: []change.Change{
				prChange(pr, trunkPatch),
				prChange(pr, trunkFeature), // same PR, two separate kept changes
			},
			allMergedPRs: []ghPullRequest{pr},
			wantCommitMap: map[string]int{
				prHash: 2,
			},
		},
		{
			name: "mix of PR and issue changes across different commits",
			keptChanges: []change.Change{
				prChange(pr, trunkPatch),
				issueChange(issue, trunkFeature),
				prChange(pr2, trunkPatch),
			},
			allMergedPRs: []ghPullRequest{pr, closingPR, pr2},
			wantCommitMap: map[string]int{
				prHash:    1,
				issueHash: 1,
				prHash2:   1,
			},
		},
		{
			name:          "empty input produces empty map",
			keptChanges:   nil,
			allMergedPRs:  nil,
			wantCommitMap: map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapKeptChangesToCommits(tt.keptChanges, tt.allMergedPRs)
			require.Len(t, got, len(tt.wantCommitMap), "number of distinct commit hashes in result")
			for hash, wantCount := range tt.wantCommitMap {
				require.Contains(t, got, hash)
				require.Len(t, got[hash], wantCount)
			}
		})
	}
}

func Test_buildTrunkPRMap(t *testing.T) {
	mergeHash := "deadbeef"
	issueURL := "https://github.com/owner/repo/issues/99"

	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: trunkBaseTime.Add(-48 * time.Hour),
	}
	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: trunkBaseTime.Add(48 * time.Hour),
	}

	tests := []struct {
		name         string
		pr           ghPullRequest
		keptByCommit map[string][]change.Change
		config       Config
		sinceTag     *git.Tag
		untilTag     *git.Tag
		// set skipCommitSet when the PR should not be in the commit set at all
		skipCommitSet       bool
		wantFiltered        bool
		wantReason          string
		wantChangeTypeNames []string // checked only when wantFiltered==false and non-nil
	}{
		{
			// the syft case: PR carries no change-type labels, but its linked issue
			// does. keptByCommit tells us which change-types were actually kept.
			name: "PR kept via linked issue (syft case — PR has no change-type label)",
			pr: ghPullRequest{
				Number:      1,
				Title:       "implement feature X",
				MergeCommit: mergeHash,
				Labels:      []string{}, // no change-type label on the PR itself
				LinkedIssues: []ghIssue{
					{Number: 99, URL: issueURL, Labels: []string{"feature"}, Closed: true},
				},
			},
			keptByCommit: makeKeptByCommit(mergeHash, issueChange(
				ghIssue{Number: 99, URL: issueURL, Labels: []string{"feature"}, Closed: true},
				trunkFeature,
			)),
			config:              Config{ChangeTypesByLabel: trunkChangeTypes},
			wantFiltered:        false,
			wantChangeTypeNames: []string{"added-feature"},
		},
		{
			// PR directly carries a change-type label and appears in keptByCommit
			name: "PR kept directly via its own label",
			pr: ghPullRequest{
				Number:      2,
				Title:       "fix regression",
				MergeCommit: mergeHash,
				Labels:      []string{"bug"},
			},
			keptByCommit: makeKeptByCommit(mergeHash, prChange(
				ghPullRequest{Number: 2, MergeCommit: mergeHash, Labels: []string{"bug"}},
				trunkPatch,
			)),
			config:              Config{ChangeTypesByLabel: trunkChangeTypes},
			wantFiltered:        false,
			wantChangeTypeNames: []string{"patch"},
		},
		{
			// PR with no kept change → filtered; reason comes from explainPRNotKept
			name: "PR filtered — no change-type label and not in keptByCommit",
			pr: ghPullRequest{
				Number:      3,
				Title:       "docs update",
				MergeCommit: mergeHash,
				Labels:      []string{"documentation"},
			},
			keptByCommit: nil,
			config:       Config{ChangeTypesByLabel: trunkChangeTypes},
			wantFiltered: true,
			wantReason:   "label:missing-required",
		},
		{
			// PR outside commitHashSet → absent from result entirely
			name: "PR outside commit set is ignored",
			pr: ghPullRequest{
				Number:      4,
				Title:       "outside-range PR",
				MergeCommit: "not-in-set",
				Labels:      []string{"bug"},
			},
			keptByCommit:  nil,
			config:        Config{ChangeTypesByLabel: trunkChangeTypes},
			skipCommitSet: true,
		},
		{
			// PR with an excluded label → filtered with "label:excluded:<name>"
			name: "PR filtered — excluded label",
			pr: ghPullRequest{
				Number:      5,
				Title:       "chore: bump deps",
				MergeCommit: mergeHash,
				Labels:      []string{"bug", "chore"},
			},
			keptByCommit: nil,
			config: Config{
				ChangeTypesByLabel: trunkChangeTypes,
				ExcludeLabels:      []string{"chore"},
			},
			wantFiltered: true,
			wantReason:   "label:excluded:chore",
		},
		{
			// PR merged before sinceTag → "chronology:too-old" from prsAfter
			name: "PR filtered — before since tag",
			pr: ghPullRequest{
				Number:      6,
				Title:       "old fix",
				MergeCommit: mergeHash,
				Labels:      []string{"bug"},
				MergedAt:    trunkBaseTime.Add(-72 * time.Hour),
			},
			keptByCommit: nil,
			config:       Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:     sinceTag,
			untilTag:     untilTag,
			wantFiltered: true,
			wantReason:   "chronology:too-old",
		},
		{
			// PR merged after untilTag → "chronology:too-new" from prsAtOrBefore
			name: "PR filtered — after until tag",
			pr: ghPullRequest{
				Number:      7,
				Title:       "future fix",
				MergeCommit: mergeHash,
				Labels:      []string{"bug"},
				MergedAt:    trunkBaseTime.Add(96 * time.Hour),
			},
			keptByCommit: nil,
			config:       Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:     sinceTag,
			untilTag:     untilTag,
			wantFiltered: true,
			wantReason:   "chronology:too-new",
		},
		{
			// ChangeTypes is the union across all kept changes for that commit
			name: "ChangeTypes unioned across multiple kept changes (patch + feature)",
			pr: ghPullRequest{
				Number:      8,
				Title:       "big PR",
				MergeCommit: mergeHash,
				Labels:      []string{},
			},
			keptByCommit: map[string][]change.Change{
				mergeHash: {
					issueChange(ghIssue{Number: 1, URL: "u1"}, trunkPatch),
					issueChange(ghIssue{Number: 2, URL: "u2"}, trunkFeature),
				},
			},
			config:              Config{ChangeTypesByLabel: trunkChangeTypes},
			wantFiltered:        false,
			wantChangeTypeNames: []string{"patch", "added-feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipCommitSet {
				// PR whose MergeCommit is not in the set should produce an empty map
				prMap := buildTrunkPRMap(tt.config, []ghPullRequest{tt.pr}, strset.New("some-other-hash"), nil, nil, nil)
				require.Empty(t, prMap)
				return
			}

			commitSet := strset.New(mergeHash)
			prMap := buildTrunkPRMap(tt.config, []ghPullRequest{tt.pr}, commitSet, tt.keptByCommit, tt.sinceTag, tt.untilTag)

			require.Contains(t, prMap, mergeHash)
			tp := prMap[mergeHash]

			if tt.wantFiltered {
				require.True(t, tp.Filtered, "expected PR to be filtered")
				require.Equal(t, tt.wantReason, tp.Reason)
			} else {
				require.False(t, tp.Filtered, "expected PR to be kept")
				require.Empty(t, tp.Reason)
				if tt.wantChangeTypeNames != nil {
					var gotNames []string
					for _, ct := range tp.ChangeTypes {
						gotNames = append(gotNames, ct.Name)
					}
					require.ElementsMatch(t, tt.wantChangeTypeNames, gotNames)
				}
			}
		})
	}
}

func Test_buildTrunkPRMap_prOutsideCommitSet(t *testing.T) {
	// a PR whose MergeCommit is not in the commit set is silently skipped
	pr := ghPullRequest{
		Number:      99,
		Title:       "some PR",
		MergeCommit: "not-in-set",
		Labels:      []string{"bug"},
	}
	commitSet := strset.New("some-other-hash")
	cfg := Config{ChangeTypesByLabel: trunkChangeTypes}

	prMap := buildTrunkPRMap(cfg, []ghPullRequest{pr}, commitSet, nil, nil, nil)

	require.Empty(t, prMap)
}

func Test_buildTrunkPRMap_directCommitNotInPRMap(t *testing.T) {
	// a commit hash that matches no PR.MergeCommit is absent from the map;
	// TrunkCommit.PR stays nil for such commits.
	directHash := "direct01"
	prHash := "pr000001"

	prs := []ghPullRequest{
		{Number: 5, Title: "a PR", MergeCommit: prHash, Labels: []string{"bug"}},
	}

	commitSet := strset.New(directHash, prHash)
	cfg := Config{ChangeTypesByLabel: trunkChangeTypes}

	keptByCommit := makeKeptByCommit(prHash, prChange(prs[0], trunkPatch))
	prMap := buildTrunkPRMap(cfg, prs, commitSet, keptByCommit, nil, nil)

	require.Contains(t, prMap, prHash)
	require.NotContains(t, prMap, directHash)
}

func Test_buildKeptTrunkPR(t *testing.T) {
	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: trunkBaseTime.Add(-24 * time.Hour),
	}
	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: trunkBaseTime.Add(24 * time.Hour),
	}

	keptIssueURL := "https://github.com/owner/repo/issues/10"
	filteredIssueURL := "https://github.com/owner/repo/issues/11"

	tests := []struct {
		name          string
		pr            ghPullRequest
		keptForCommit []change.Change
		config        Config
		sinceTag      *git.Tag
		untilTag      *git.Tag
		// wantNoIssues is true when Issues should be nil or empty (open issues
		// are skipped, returning an empty non-nil slice; no linked issues returns nil)
		wantNoIssues     bool
		wantIssueCount   int
		wantChangeTypes  []string // expected type names, order-independent
		checkIssueDetail func(t *testing.T, issues []release.TrunkIssue)
	}{
		{
			name: "kept PR with no linked issues — Issues is nil",
			pr: ghPullRequest{
				Number:      1,
				Title:       "fix via PR labels",
				URL:         "https://github.com/owner/repo/pull/1",
				Author:      "alice",
				MergeCommit: "hash1",
				Labels:      []string{"bug"},
			},
			keptForCommit: []change.Change{
				prChange(ghPullRequest{Number: 1, MergeCommit: "hash1", Labels: []string{"bug"}}, trunkPatch),
			},
			config:          Config{ChangeTypesByLabel: trunkChangeTypes},
			wantNoIssues:    true,
			wantChangeTypes: []string{"patch"},
		},
		{
			name: "kept PR with kept linked issue — Issues contains TrunkIssue with Filtered=false",
			pr: ghPullRequest{
				Number:      2,
				Title:       "implement feature",
				URL:         "https://github.com/owner/repo/pull/2",
				Author:      "bob",
				MergeCommit: "hash2",
				Labels:      []string{},
				LinkedIssues: []ghIssue{
					{
						Number:   10,
						Title:    "add feature X",
						URL:      keptIssueURL,
						Labels:   []string{"feature"},
						Closed:   true,
						ClosedAt: trunkBaseTime,
					},
				},
			},
			keptForCommit: []change.Change{
				issueChange(ghIssue{Number: 10, URL: keptIssueURL, Labels: []string{"feature"}, Closed: true}, trunkFeature),
			},
			config:          Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:        sinceTag,
			untilTag:        untilTag,
			wantNoIssues:    false,
			wantIssueCount:  1,
			wantChangeTypes: []string{"added-feature"},
			checkIssueDetail: func(t *testing.T, issues []release.TrunkIssue) {
				t.Helper()
				require.False(t, issues[0].Filtered)
				require.Equal(t, 10, issues[0].Number)
				require.Empty(t, issues[0].Reason)
			},
		},
		{
			// closed linked issue that was NOT included in keptForCommit →
			// TrunkIssue with Filtered=true and a non-empty Reason
			name: "kept PR with closed-but-not-kept linked issue — Issues contains Filtered TrunkIssue",
			pr: ghPullRequest{
				Number:      3,
				Title:       "fix something",
				MergeCommit: "hash3",
				Labels:      []string{"bug"},
				LinkedIssues: []ghIssue{
					{
						Number:   11,
						Title:    "excluded issue",
						URL:      filteredIssueURL,
						Labels:   []string{"bug", "chore"},
						Closed:   true,
						ClosedAt: trunkBaseTime,
					},
				},
			},
			// keptForCommit references the PR itself, not the issue
			keptForCommit: []change.Change{
				prChange(ghPullRequest{Number: 3, MergeCommit: "hash3", Labels: []string{"bug"}}, trunkPatch),
			},
			config: Config{
				ChangeTypesByLabel: trunkChangeTypes,
				ExcludeLabels:      []string{"chore"},
			},
			sinceTag:       sinceTag,
			untilTag:       untilTag,
			wantNoIssues:   false,
			wantIssueCount: 1,
			checkIssueDetail: func(t *testing.T, issues []release.TrunkIssue) {
				t.Helper()
				require.True(t, issues[0].Filtered)
				require.NotEmpty(t, issues[0].Reason)
			},
		},
		{
			// open linked issue is skipped by buildKeptTrunkIssues; the function
			// returns an empty (non-nil) slice in this case
			name: "kept PR with only open linked issue — Issues is empty",
			pr: ghPullRequest{
				Number:      4,
				Title:       "partial implementation",
				MergeCommit: "hash4",
				Labels:      []string{"bug"},
				LinkedIssues: []ghIssue{
					{Number: 12, Title: "open issue", URL: "https://github.com/owner/repo/issues/12", Labels: []string{"bug"}, Closed: false},
				},
			},
			keptForCommit: []change.Change{
				prChange(ghPullRequest{Number: 4, MergeCommit: "hash4", Labels: []string{"bug"}}, trunkPatch),
			},
			config:          Config{ChangeTypesByLabel: trunkChangeTypes},
			wantNoIssues:    true,
			wantChangeTypes: []string{"patch"},
		},
		{
			// ChangeTypes is the union of types from all kept changes for the commit
			name: "ChangeTypes unioned from two kept issues with different types",
			pr: ghPullRequest{
				Number:      5,
				Title:       "big feature+fix PR",
				MergeCommit: "hash5",
				Labels:      []string{},
				LinkedIssues: []ghIssue{
					{Number: 20, Title: "bug", URL: "https://github.com/owner/repo/issues/20", Labels: []string{"bug"}, Closed: true, ClosedAt: trunkBaseTime},
					{Number: 21, Title: "feat", URL: "https://github.com/owner/repo/issues/21", Labels: []string{"feature"}, Closed: true, ClosedAt: trunkBaseTime},
				},
			},
			keptForCommit: []change.Change{
				issueChange(ghIssue{Number: 20, URL: "https://github.com/owner/repo/issues/20"}, trunkPatch),
				issueChange(ghIssue{Number: 21, URL: "https://github.com/owner/repo/issues/21"}, trunkFeature),
			},
			config:          Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:        sinceTag,
			untilTag:        untilTag,
			wantNoIssues:    false,
			wantIssueCount:  2,
			wantChangeTypes: []string{"patch", "added-feature"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKeptTrunkPR(tt.config, tt.pr, tt.keptForCommit, tt.sinceTag, tt.untilTag)

			require.NotNil(t, got)
			require.False(t, got.Filtered)

			if tt.wantNoIssues {
				require.Empty(t, got.Issues)
			} else {
				require.Len(t, got.Issues, tt.wantIssueCount)
			}

			if tt.wantChangeTypes != nil {
				var gotNames []string
				for _, ct := range got.ChangeTypes {
					gotNames = append(gotNames, ct.Name)
				}
				require.ElementsMatch(t, tt.wantChangeTypes, gotNames)
			}

			if tt.checkIssueDetail != nil {
				tt.checkIssueDetail(t, got.Issues)
			}
		})
	}
}

func Test_buildKeptTrunkIssues(t *testing.T) {
	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: trunkBaseTime.Add(-24 * time.Hour),
	}
	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: trunkBaseTime.Add(24 * time.Hour),
	}

	keptURL := "https://github.com/owner/repo/issues/1"
	filteredURL := "https://github.com/owner/repo/issues/2"

	tests := []struct {
		name     string
		issues   []ghIssue
		keptURLs map[string]bool
		config   Config
		// wantEmpty is true when the result should be nil or empty
		wantEmpty  bool
		wantResult []release.TrunkIssue
	}{
		{
			name:      "empty input returns nil",
			issues:    nil,
			wantEmpty: true,
		},
		{
			// open issues are skipped; the pre-allocated slice is returned empty
			name: "open issue is skipped — result is empty",
			issues: []ghIssue{
				{Number: 3, Title: "still open", URL: "u3", Labels: []string{"bug"}, Closed: false},
			},
			keptURLs:  map[string]bool{},
			config:    Config{ChangeTypesByLabel: trunkChangeTypes},
			wantEmpty: true,
		},
		{
			name: "kept issue — Filtered=false with change types from config",
			issues: []ghIssue{
				{Number: 1, Title: "bug fix", URL: keptURL, Labels: []string{"bug"}, Closed: true, ClosedAt: trunkBaseTime},
			},
			keptURLs: map[string]bool{keptURL: true},
			config:   Config{ChangeTypesByLabel: trunkChangeTypes},
			wantResult: []release.TrunkIssue{
				{Number: 1, Title: "bug fix", URL: keptURL, Labels: []string{"bug"}, ChangeTypes: []change.Type{trunkPatch}, Filtered: false},
			},
		},
		{
			name: "closed-but-not-kept issue — Filtered=true with non-empty Reason",
			issues: []ghIssue{
				{Number: 2, Title: "excluded", URL: filteredURL, Labels: []string{"bug", "chore"}, Closed: true, ClosedAt: trunkBaseTime},
			},
			keptURLs: map[string]bool{},
			config: Config{
				ChangeTypesByLabel: trunkChangeTypes,
				ExcludeLabels:      []string{"chore"},
			},
			wantResult: []release.TrunkIssue{
				{Number: 2, Title: "excluded", URL: filteredURL, Labels: []string{"bug", "chore"}, Filtered: true, Reason: "label:excluded:chore"},
			},
		},
		{
			name: "kept and filtered in same slice are each classified independently",
			issues: []ghIssue{
				{Number: 1, Title: "good", URL: keptURL, Labels: []string{"bug"}, Closed: true, ClosedAt: trunkBaseTime},
				{Number: 2, Title: "excluded", URL: filteredURL, Labels: []string{"bug", "chore"}, Closed: true, ClosedAt: trunkBaseTime},
			},
			keptURLs: map[string]bool{keptURL: true},
			config: Config{
				ChangeTypesByLabel: trunkChangeTypes,
				ExcludeLabels:      []string{"chore"},
			},
			wantResult: []release.TrunkIssue{
				{Number: 1, Title: "good", URL: keptURL, Labels: []string{"bug"}, ChangeTypes: []change.Type{trunkPatch}, Filtered: false},
				{Number: 2, Title: "excluded", URL: filteredURL, Labels: []string{"bug", "chore"}, Filtered: true, Reason: "label:excluded:chore"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildKeptTrunkIssues(tt.config, tt.issues, tt.keptURLs, sinceTag, untilTag)

			if tt.wantEmpty {
				require.Empty(t, got)
				return
			}

			if diff := cmp.Diff(tt.wantResult, got, sortChangeTypes, sortTrunkIssues); diff != "" {
				t.Errorf("buildKeptTrunkIssues mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func Test_explainPRNotKept(t *testing.T) {
	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: trunkBaseTime.Add(-48 * time.Hour),
	}
	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: trunkBaseTime.Add(48 * time.Hour),
	}

	tests := []struct {
		name       string
		pr         ghPullRequest
		config     Config
		sinceTag   *git.Tag
		untilTag   *git.Tag
		wantReason string
	}{
		{
			name: "PR merged before since tag — chronology:too-old",
			pr: ghPullRequest{
				Number:   1,
				Labels:   []string{"bug"},
				MergedAt: trunkBaseTime.Add(-96 * time.Hour),
			},
			config:     Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:   sinceTag,
			untilTag:   untilTag,
			wantReason: "chronology:too-old",
		},
		{
			name: "PR merged after until tag — chronology:too-new",
			pr: ghPullRequest{
				Number:   2,
				Labels:   []string{"bug"},
				MergedAt: trunkBaseTime.Add(96 * time.Hour),
			},
			config:     Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:   sinceTag,
			untilTag:   untilTag,
			wantReason: "chronology:too-new",
		},
		{
			name: "PR with excluded label — label:excluded:<name>",
			pr: ghPullRequest{
				Number:   3,
				Labels:   []string{"bug", "chore"},
				MergedAt: trunkBaseTime,
			},
			config: Config{
				ChangeTypesByLabel: trunkChangeTypes,
				ExcludeLabels:      []string{"chore"},
			},
			wantReason: "label:excluded:chore",
		},
		{
			// PR has a closed linked issue whose issue lacks a change-type label;
			// contribution path is "via linked issue" so reason is prefixed "linked-issue:"
			name: "PR with closed linked issue that lacks change-type — linked-issue:label:missing-required",
			pr: ghPullRequest{
				Number:   4,
				Labels:   []string{}, // no change-type on the PR
				MergedAt: trunkBaseTime,
				LinkedIssues: []ghIssue{
					{Number: 99, Labels: []string{"documentation"}, Closed: true, ClosedAt: trunkBaseTime},
				},
			},
			config:     Config{ChangeTypesByLabel: trunkChangeTypes},
			sinceTag:   sinceTag,
			untilTag:   untilTag,
			wantReason: "linked-issue:label:missing-required",
		},
		{
			// PR has no closed linked issue and no change-type label → direct path fails
			name: "PR has no closed linked issue and no change-type label — label:missing-required",
			pr: ghPullRequest{
				Number:   5,
				Labels:   []string{"documentation"},
				MergedAt: trunkBaseTime,
			},
			config:     Config{ChangeTypesByLabel: trunkChangeTypes},
			wantReason: "label:missing-required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainPRNotKept(tt.pr, tt.config, tt.sinceTag, tt.untilTag)
			require.Equal(t, tt.wantReason, got)
		})
	}
}

func Test_explainIssueNotKept(t *testing.T) {
	sinceTag := &git.Tag{
		Name:      "v0.1.0",
		Timestamp: trunkBaseTime.Add(-48 * time.Hour),
	}
	untilTag := &git.Tag{
		Name:      "v0.2.0",
		Timestamp: trunkBaseTime.Add(48 * time.Hour),
	}

	tests := []struct {
		name       string
		issue      ghIssue
		config     Config
		wantReason string
	}{
		{
			name: "issue with excluded label",
			issue: ghIssue{
				Number:   1,
				Labels:   []string{"bug", "chore"},
				ClosedAt: trunkBaseTime,
				Closed:   true,
			},
			config: Config{
				ChangeTypesByLabel: trunkChangeTypes,
				ExcludeLabels:      []string{"chore"},
			},
			// standardIssueFilters puts issuesWithLabel first; "bug" passes it.
			// then issuesWithoutLabel fires on "chore"
			wantReason: "label:excluded:chore",
		},
		{
			// issue with no recognised change-type label fails issuesWithLabel first
			name: "issue without change-type label — label:missing-required",
			issue: ghIssue{
				Number:   2,
				Labels:   []string{"documentation"},
				ClosedAt: trunkBaseTime,
				Closed:   true,
			},
			config:     Config{ChangeTypesByLabel: trunkChangeTypes},
			wantReason: "label:missing-required",
		},
		{
			// "bug" label passes issuesWithLabel; then issuesAfter fires because
			// the issue was closed before the sinceTag
			name: "issue closed before since tag — chronology:before-since",
			issue: ghIssue{
				Number:   3,
				Labels:   []string{"bug"},
				ClosedAt: trunkBaseTime.Add(-96 * time.Hour),
				Closed:   true,
			},
			config:     Config{ChangeTypesByLabel: trunkChangeTypes},
			wantReason: "chronology:before-since",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := explainIssueNotKept(tt.issue, tt.config, sinceTag, untilTag)
			require.Equal(t, tt.wantReason, got)
		})
	}
}

func Test_hasClosedLinkedIssue(t *testing.T) {
	tests := []struct {
		name string
		pr   ghPullRequest
		want bool
	}{
		{
			name: "PR with closed linked issue — true",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{Number: 1, Closed: true},
				},
			},
			want: true,
		},
		{
			name: "PR with only open linked issue — false",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{Number: 2, Closed: false},
				},
			},
			want: false,
		},
		{
			name: "PR with no linked issues — false",
			pr:   ghPullRequest{},
			want: false,
		},
		{
			name: "PR with mix of open and closed — true (has at least one closed)",
			pr: ghPullRequest{
				LinkedIssues: []ghIssue{
					{Number: 3, Closed: false},
					{Number: 4, Closed: true},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasClosedLinkedIssue(tt.pr)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_hasChangeTypeLabel(t *testing.T) {
	cfg := Config{ChangeTypesByLabel: trunkChangeTypes}

	tests := []struct {
		name   string
		labels []string
		want   bool
	}{
		{
			name:   "label matching a change type key — true",
			labels: []string{"bug"},
			want:   true,
		},
		{
			name:   "another matching label — true",
			labels: []string{"feature"},
			want:   true,
		},
		{
			name:   "no matching label — false",
			labels: []string{"documentation", "chore"},
			want:   false,
		},
		{
			name:   "empty labels — false",
			labels: nil,
			want:   false,
		},
		{
			name:   "mix of matching and non-matching — true",
			labels: []string{"documentation", "bug"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasChangeTypeLabel(tt.labels, cfg)
			require.Equal(t, tt.want, got)
		})
	}
}
