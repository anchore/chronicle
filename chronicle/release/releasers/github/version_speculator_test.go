package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
)

func TestFindNextVersion(t *testing.T) {
	majorChange := change.Type{
		Kind: change.SemVerMajor,
	}

	minorChange := change.Type{
		Kind: change.SemVerMinor,
	}

	patchChange := change.Type{
		Kind: change.SemVerPatch,
	}

	tests := []struct {
		name                string
		release             string
		changes             change.Changes
		enforceV0           bool
		bumpPatchOnNoChange bool
		want                string
		wantErr             require.ErrorAssertionFunc
	}{
		{
			name:    "bump major version",
			release: "v0.1.5",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange, minorChange, patchChange},
				},
			},
			want: "v1.0.0",
		},
		{
			name:      "bump major version -- enforce v0",
			release:   "v0.1.5",
			enforceV0: true,
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange, minorChange, patchChange},
				},
			},
			want: "v0.2.0",
		},
		{
			name:      "bump major version -- enforce v0 -- keep major",
			release:   "v6.1.5",
			enforceV0: true,
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange, minorChange, patchChange},
				},
			},
			want: "v6.2.0",
		},
		{
			name:    "bump major version -- ignore dups",
			release: "v0.1.5",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange, majorChange, majorChange, majorChange, majorChange, majorChange},
				},
			},
			want: "v1.0.0",
		},
		{
			name:    "bump minor version",
			release: "v0.1.5",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange, patchChange},
				},
			},
			want: "v0.2.0",
		},
		{
			name:    "bump patch version",
			release: "v0.1.5",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{patchChange},
				},
			},
			want: "v0.1.6",
		},
		{
			name:    "honor no prefix",
			release: "0.1.5",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{patchChange},
				},
			},
			want: "0.1.6",
		},
		{
			name:                "no changes -- bump patch",
			release:             "0.1.5",
			bumpPatchOnNoChange: true,
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{},
				},
			},
			want: "0.1.6",
		},
		{
			name:                "no changes -- error",
			release:             "0.1.5",
			bumpPatchOnNoChange: false,
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{},
				},
			},
			wantErr: require.Error,
		},
		{
			name:    "error on bad version",
			release: "a10",
			wantErr: require.Error,
		},
		{
			name:    "empty version with minor change",
			release: "",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange},
				},
			},
			want: "v0.1.0",
		},
		{
			name:    "empty version with patch change",
			release: "",
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{patchChange},
				},
			},
			want: "v0.0.1",
		},
		{
			name:                "empty version with no changes -- bump patch",
			release:             "",
			bumpPatchOnNoChange: true,
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{},
				},
			},
			want: "v0.0.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			s := NewVersionSpeculator(nil, release.SpeculationBehavior{
				EnforceV0:           tt.enforceV0,
				NoChangesBumpsPatch: tt.bumpPatchOnNoChange,
			})

			got, err := s.NextIdealVersion(tt.release, tt.changes)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindNextUniqueVersion(t *testing.T) {
	majorChange := change.Type{
		Kind: change.SemVerMajor,
	}

	minorChange := change.Type{
		Kind: change.SemVerMinor,
	}

	patchChange := change.Type{
		Kind: change.SemVerPatch,
	}

	tests := []struct {
		name                string
		release             string
		git                 git.Interface
		changes             change.Changes
		enforceV0           bool
		bumpPatchOnNoChange bool
		want                string
		wantErr             require.ErrorAssertionFunc
	}{
		{
			// a taken major version rolls forward by minor, never by major (we never
			// speculate a brand new major version) and never by patch.
			name:    "bump major version -- major conflict rolls forward by minor",
			release: "v1.5.2",
			git: git.MockInterface{
				MockTags: []string{
					"v2.0.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange, minorChange, patchChange},
				},
			},
			want: "v2.1.0",
		},
		{
			name:    "bump major version -- multiple major conflicts roll forward by minor",
			release: "v1.5.2",
			git: git.MockInterface{
				MockTags: []string{
					"v2.0.0",
					"v2.1.0",
					"v2.2.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange},
				},
			},
			want: "v2.3.0",
		},
		{
			// a taken feature (minor) version rolls forward by minor, not patch.
			name:    "bump feature version -- minor conflict rolls forward by minor",
			release: "v0.12.0",
			git: git.MockInterface{
				MockTags: []string{
					"v0.13.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange},
				},
			},
			want: "v0.14.0",
		},
		{
			name:    "bump feature version -- multiple minor conflicts roll forward by minor",
			release: "v0.12.0",
			git: git.MockInterface{
				MockTags: []string{
					"v0.13.0",
					"v0.14.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange},
				},
			},
			want: "v0.15.0",
		},
		{
			// a taken patch version rolls forward by patch.
			name:    "bump patch version -- patch conflict rolls forward by patch",
			release: "v0.13.0",
			git: git.MockInterface{
				MockTags: []string{
					"v0.13.1",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{patchChange},
				},
			},
			want: "v0.13.2",
		},
		{
			name:    "no conflict returns the ideal version",
			release: "v0.12.0",
			git: git.MockInterface{
				MockTags: []string{
					"v0.12.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange},
				},
			},
			want: "v0.13.0",
		},
		{
			// breaking changes on a v0.x project with EnforceV0 are minor bumps, so a
			// conflict rolls forward by minor.
			name:      "bump major version -- enforce v0 conflict rolls forward by minor",
			release:   "v0.12.0",
			enforceV0: true,
			git: git.MockInterface{
				MockTags: []string{
					"v0.13.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{majorChange},
				},
			},
			want: "v0.14.0",
		},
		{
			// when there are no existing tags the ideal version is returned unchanged.
			name:    "no tags returns the ideal version",
			release: "v0.12.0",
			git:     git.MockInterface{},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange},
				},
			},
			want: "v0.13.0",
		},
		{
			// multiple consecutive patch conflicts roll forward by patch.
			name:    "bump patch version -- multiple patch conflicts roll forward by patch",
			release: "v0.13.0",
			git: git.MockInterface{
				MockTags: []string{
					"v0.13.1",
					"v0.13.2",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{patchChange},
				},
			},
			want: "v0.13.3",
		},
		{
			// a version without a "v" prefix keeps its prefix-less form when rolling forward.
			name:    "no prefix conflict rolls forward without prefix",
			release: "0.12.0",
			git: git.MockInterface{
				MockTags: []string{
					"0.13.0",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{minorChange},
				},
			},
			want: "0.14.0",
		},
		{
			// no changes with NoChangesBumpsPatch yields a patch bump, so a conflict rolls
			// forward by patch (the default bump kind for an unknown significance).
			name:                "no changes bump patch conflict rolls forward by patch",
			release:             "v0.13.0",
			bumpPatchOnNoChange: true,
			git: git.MockInterface{
				MockTags: []string{
					"v0.13.1",
				},
			},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{},
				},
			},
			want: "v0.13.2",
		},
		{
			// an error from the underlying ideal-version computation is propagated.
			name:    "error on bad version is propagated",
			release: "a10",
			git:     git.MockInterface{},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{patchChange},
				},
			},
			wantErr: require.Error,
		},
		{
			// no changes without NoChangesBumpsPatch errors out at the ideal-version step.
			name:    "no changes without bump patch errors",
			release: "v0.13.0",
			git:     git.MockInterface{},
			changes: []change.Change{
				{
					ChangeTypes: []change.Type{},
				},
			},
			wantErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			s := NewVersionSpeculator(tt.git, release.SpeculationBehavior{
				EnforceV0:           tt.enforceV0,
				NoChangesBumpsPatch: tt.bumpPatchOnNoChange,
			})

			got, err := s.NextUniqueVersion(tt.release, tt.changes)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
