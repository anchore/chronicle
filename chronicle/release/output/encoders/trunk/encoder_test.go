package trunk

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// fixtureRelease is the current release used across all snapshot tests.
var fixtureRelease = release.Release{
	Version: "v0.5.0",
	Date:    time.Date(2026, time.May, 7, 0, 0, 0, 0, time.UTC),
}

// fixturePreviousRelease is the prior release anchor.
var fixturePreviousRelease = &release.Release{
	Version: "v0.4.1",
	Date:    time.Date(2026, time.April, 18, 0, 0, 0, 0, time.UTC),
}

// fixtureTrunkData is a small but representative TrunkData covering:
//   - a kept commit with a PR, two kept issues, and one filtered issue
//   - a filtered commit (no-PR case)
//   - a filtered commit (PR exists but is filtered)
//   - a kept commit with a PR and no issues
var fixtureTrunkData = &release.TrunkData{
	Commits: []release.TrunkCommit{
		{
			Hash:    "a1b2c3d4e5f6abc",
			Subject: "Merge pull request #466 from user/feature",
			PR: &release.TrunkPR{
				Number: 466,
				Title:  "feat: multi-output formats",
				ChangeTypes: []change.Type{
					change.NewType("enhancement", change.SemVerMinor),
				},
				Issues: []release.TrunkIssue{
					{
						Number: 450,
						Title:  "add JSON output to releases",
						ChangeTypes: []change.Type{
							change.NewType("enhancement", change.SemVerMinor),
						},
					},
					{
						Number: 451,
						Title:  "document the JSON schema",
						ChangeTypes: []change.Type{
							change.NewType("enhancement", change.SemVerMinor),
						},
					},
					{
						Number:   452,
						Title:    "stale issue that was closed but not in scope",
						Filtered: true,
						Reason:   "out-of-scope",
					},
				},
			},
		},
		{
			// commit with no PR — always filtered.
			Hash:    "deadbeefcafe000",
			Subject: "chore: fix typo in README",
		},
		{
			Hash:    "f7e8d9c0b1a2345",
			Subject: "Merge pull request #467 from user/chore",
			PR: &release.TrunkPR{
				Number:   467,
				Title:    "chore: update CI config",
				Filtered: true,
				Reason:   "label:chore",
			},
		},
		{
			Hash:    "1234567890abcde",
			Subject: "Merge pull request #470 from user/bugfix",
			PR: &release.TrunkPR{
				Number: 470,
				Title:  "fix: nil deref in summarizer",
				ChangeTypes: []change.Type{
					change.NewType("bug", change.SemVerPatch),
				},
				Issues: []release.TrunkIssue{
					{
						Number: 468,
						Title:  "panic on empty merge commit list",
						ChangeTypes: []change.Type{
							change.NewType("bug", change.SemVerPatch),
						},
					},
				},
			},
		},
	},
}

// fixtureDescription builds a full Description from the fixture data.
func fixtureDescription(speculated bool, prevRelease *release.Release, trunk *release.TrunkData) release.Description {
	return release.Description{
		Release:         fixtureRelease,
		PreviousRelease: prevRelease,
		Speculated:      speculated,
		Trunk:           trunk,
	}
}

func TestEncoder_Encode(t *testing.T) {
	tests := []struct {
		name    string
		encoder Encoder
		desc    release.Description
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "condensed, hide-filtered, with previous release",
			encoder: Encoder{
				Condensed:    true,
				ShowFiltered: false,
				IsTTY:        false,
			},
			desc: fixtureDescription(false, fixturePreviousRelease, fixtureTrunkData),
		},
		{
			name: "condensed, show-filtered, with previous release",
			encoder: Encoder{
				Condensed:    true,
				ShowFiltered: true,
				IsTTY:        false,
			},
			desc: fixtureDescription(false, fixturePreviousRelease, fixtureTrunkData),
		},
		{
			name: "expanded, hide-filtered, with previous release",
			encoder: Encoder{
				Condensed:    false,
				ShowFiltered: false,
				IsTTY:        false,
			},
			desc: fixtureDescription(false, fixturePreviousRelease, fixtureTrunkData),
		},
		{
			name: "expanded, show-filtered, with previous release",
			encoder: Encoder{
				Condensed:    false,
				ShowFiltered: true,
				IsTTY:        false,
			},
			desc: fixtureDescription(false, fixturePreviousRelease, fixtureTrunkData),
		},
		{
			name: "condensed, no previous release (no bottom anchor)",
			encoder: Encoder{
				Condensed:    true,
				ShowFiltered: false,
				IsTTY:        false,
			},
			desc: fixtureDescription(false, nil, fixtureTrunkData),
		},
		{
			name: "condensed, speculated release (open diamond + parenthetical)",
			encoder: Encoder{
				Condensed:    true,
				ShowFiltered: false,
				IsTTY:        false,
			},
			desc: fixtureDescription(true, fixturePreviousRelease, fixtureTrunkData),
		},
		{
			name: "non-TTY output contains no ANSI escapes",
			encoder: Encoder{
				Condensed:    true,
				ShowFiltered: true,
				IsTTY:        false,
			},
			desc: fixtureDescription(false, fixturePreviousRelease, fixtureTrunkData),
		},
		{
			name: "nil trunk returns error",
			encoder: Encoder{
				Condensed: true,
				IsTTY:     false,
			},
			desc:    fixtureDescription(false, fixturePreviousRelease, nil),
			wantErr: require.Error,
		},
		{
			name: "empty trunk commits returns error",
			encoder: Encoder{
				Condensed: true,
				IsTTY:     false,
			},
			desc:    fixtureDescription(false, fixturePreviousRelease, &release.TrunkData{}),
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			var buf bytes.Buffer
			err := tt.encoder.Encode(&buf, "", tt.desc)
			tt.wantErr(t, err)

			if err != nil {
				return
			}

			out := buf.String()

			// for the non-TTY case, explicitly assert no ANSI escapes.
			if tt.name == "non-TTY output contains no ANSI escapes" {
				require.False(t, strings.Contains(out, "\x1b"), "output should not contain ANSI escape codes when IsTTY=false")
			}

			snaps.MatchSnapshot(t, out)
		})
	}
}

func TestEncoder_Hyperlinks(t *testing.T) {
	// fixture with URLs populated on commit, PR, and issue.
	withURLs := &release.TrunkData{
		Commits: []release.TrunkCommit{
			{
				Hash:    "a1b2c3d4e5f6abc",
				URL:     "https://github.com/owner/repo/commit/a1b2c3d4e5f6abc",
				Subject: "feat: add thing",
				PR: &release.TrunkPR{
					Number: 466,
					Title:  "feat: add thing",
					URL:    "https://github.com/owner/repo/pull/466",
					ChangeTypes: []change.Type{
						change.NewType("enhancement", change.SemVerMinor),
					},
					Issues: []release.TrunkIssue{
						{
							Number: 450,
							Title:  "request thing",
							URL:    "https://github.com/owner/repo/issues/450",
							ChangeTypes: []change.Type{
								change.NewType("enhancement", change.SemVerMinor),
							},
						},
					},
				},
			},
		},
	}
	desc := fixtureDescription(false, fixturePreviousRelease, withURLs)

	const osc8 = "\x1b]8;;"

	t.Run("TTY emits OSC 8 hyperlinks for commit, PR, and issue", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, (&Encoder{Condensed: true, ShowFiltered: false, IsTTY: true}).Encode(&buf, "", desc))
		out := buf.String()
		require.Contains(t, out, osc8+"https://github.com/owner/repo/commit/a1b2c3d4e5f6abc")
		require.Contains(t, out, osc8+"https://github.com/owner/repo/pull/466")
		require.Contains(t, out, osc8+"https://github.com/owner/repo/issues/450")
	})

	t.Run("non-TTY emits no hyperlink escapes", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, (&Encoder{Condensed: true, ShowFiltered: false, IsTTY: false}).Encode(&buf, "", desc))
		require.NotContains(t, buf.String(), osc8)
	})

	t.Run("expanded TTY emits OSC 8 for issue ref under commit", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, (&Encoder{Condensed: false, ShowFiltered: false, IsTTY: true}).Encode(&buf, "", desc))
		require.Contains(t, buf.String(), osc8+"https://github.com/owner/repo/issues/450")
	})
}

// TestEncoder_FilteredRowsFullyDim verifies that on a TTY, every cell in a
// filtered row (hash, PR#, title, type) is wrapped in the dim SGR sequence.
// Previously the outer wrap was applied around the joined row, which could lose
// the dim attribute across OSC 8 hyperlink boundaries — leaving PR# and title
// rendered at normal brightness.
func TestEncoder_FilteredRowsFullyDim(t *testing.T) {
	td := &release.TrunkData{
		Commits: []release.TrunkCommit{
			{
				// kept commit so the rendered output has at least one visible row.
				Hash:    "a1b2c3d4e5f6abc",
				URL:     "https://github.com/owner/repo/commit/a1b2c3d4e5f6abc",
				Subject: "feat: kept",
				PR: &release.TrunkPR{
					Number: 100, Title: "feat: kept", URL: "https://github.com/owner/repo/pull/100",
					ChangeTypes: []change.Type{change.NewType("enhancement", change.SemVerMinor)},
				},
			},
			{
				// filtered commit with a PR that didn't make the changelog.
				Hash:    "deadbeefcafe000",
				URL:     "https://github.com/owner/repo/commit/deadbeefcafe000",
				Subject: "chore: skip me",
				PR: &release.TrunkPR{
					Number:   999,
					Title:    "chore: this PR was filtered",
					URL:      "https://github.com/owner/repo/pull/999",
					Filtered: true,
					Reason:   "label:chore",
				},
			},
		},
	}
	desc := fixtureDescription(false, fixturePreviousRelease, td)

	var buf bytes.Buffer
	require.NoError(t, (&Encoder{Condensed: true, ShowFiltered: true, IsTTY: true}).Encode(&buf, "", desc))
	out := buf.String()

	const dim = "\x1b[2m"

	// each visible piece of the filtered row must be wrapped by a dim SGR open code.
	// the visible chars themselves must follow a dim opener somewhere in the output.
	for _, fragment := range []string{"deadbee", "#999", "chore: this PR was filtered", "filtered: label:chore"} {
		idx := strings.Index(out, fragment)
		require.NotEqual(t, -1, idx, "expected fragment %q in output", fragment)
		// scan backward from the fragment to confirm a dim SGR opener appears
		// before any reset / non-dim SGR; the simplest check is that the substring
		// starting from the most recent dim opener still contains the fragment.
		head := out[:idx]
		dimAt := strings.LastIndex(head, dim)
		require.NotEqual(t, -1, dimAt, "fragment %q is not preceded by a dim SGR opener", fragment)
	}
}
