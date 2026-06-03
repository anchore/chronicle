package change

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ParseConventionalCommit(t *testing.T) {
	tests := []struct {
		name         string
		title        string
		wantType     string
		wantBreaking bool
		wantOk       bool
	}{
		{
			name:     "simple feat",
			title:    "feat: add user authentication",
			wantType: "feat",
			wantOk:   true,
		},
		{
			name:     "fix with scope",
			title:    "fix(parser): resolve null pointer",
			wantType: "fix",
			wantOk:   true,
		},
		{
			name:         "breaking feat via bang",
			title:        "feat!: drop legacy API",
			wantType:     "feat",
			wantBreaking: true,
			wantOk:       true,
		},
		{
			name:         "breaking with scope and bang",
			title:        "fix(api)!: change response shape",
			wantType:     "fix",
			wantBreaking: true,
			wantOk:       true,
		},
		{
			name:     "perf",
			title:    "perf: speed up lookups",
			wantType: "perf",
			wantOk:   true,
		},
		{
			name:     "free-form custom type parses",
			title:    "wibble: a custom prefix",
			wantType: "wibble",
			wantOk:   true,
		},
		{
			name:   "not a conventional commit",
			title:  "Bump foo to v2",
			wantOk: false,
		},
		{
			name:   "type without colon",
			title:  "fix",
			wantOk: false,
		},
		{
			name:   "empty description",
			title:  "feat(scope):   ",
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotBreaking, gotOk := ParseConventionalCommit(tt.title)
			assert.Equal(t, tt.wantOk, gotOk)

			if !gotOk {
				return
			}
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantBreaking, gotBreaking)
		})
	}
}

func Test_TrimConventionalCommitPrefix(t *testing.T) {
	tests := []struct {
		title      string
		recognized []string
		want       string
	}{
		// positive cases
		{title: "feat: add user authentication", want: "add user authentication"},
		{title: "fix: resolve null pointer exception", want: "resolve null pointer exception"},
		{title: "docs: update README", want: "update README"},
		{title: "style: format code according to style guide", want: "format code according to style guide"},
		{title: "refactor: extract reusable function", want: "extract reusable function"},
		{title: "perf: optimize database queries", want: "optimize database queries"},
		{title: "test: add unit tests", want: "add unit tests"},
		{title: "build: update build process", want: "update build process"},
		{title: "ci: configure Travis CI", want: "configure Travis CI"},
		{title: "chore: perform maintenance tasks", want: "perform maintenance tasks"},
		// positive case odd balls
		{title: "chore: can end with punctuation.", want: "can end with punctuation."},
		{title: "revert!: revert: previous: commit", want: "revert: previous: commit"},
		{title: "feat(api)!: implement new: API endpoints", want: "implement new: API endpoints"},
		{title: "feat!: add awesome new feature (closes #123)", want: "add awesome new feature (closes #123)"},
		{title: "fix(ui): fix layout issue (fixes #456)", want: "fix layout issue (fixes #456)"},
		// negative cases (only standard conventional types are recognized)
		{title: "reallycoolthing: is done!", want: "reallycoolthing: is done!"},
		{title: "feature: is done!", want: "feature: is done!"},
		{title: "feat(scope):   ", want: "feat(scope):   "},
		{title: "feat(scope):something", want: "feat(scope):something"},
		{title: "feat: something\n wicked this way comes", want: "feat: something\n wicked this way comes"},
		// recognized (non-standard) prefixes are trimmed, matching categorization
		{title: "deps: bump foo to v2", recognized: []string{"deps"}, want: "bump foo to v2"},
		{title: "deps(go)!: bump foo to v2", recognized: []string{"deps"}, want: "bump foo to v2"},
		{title: "security: patch CVE-2026-0001", recognized: []string{"deps", "security"}, want: "patch CVE-2026-0001"},
		// recognized matching is case-insensitive
		{title: "Deps: bump foo", recognized: []string{"deps"}, want: "bump foo"},
		{title: "deps: bump foo", recognized: []string{"Deps"}, want: "bump foo"},
		// an unrecognized non-standard prefix is left intact even when a recognized set is given
		{title: "wibble: do a thing", recognized: []string{"deps"}, want: "wibble: do a thing"},
		// standard types are always trimmed regardless of the recognized set
		{title: "feat: add a thing", recognized: []string{"deps"}, want: "add a thing"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.want, TrimConventionalCommitPrefix(tt.title, tt.recognized...))
		})
	}
}
