package release

import (
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := FindNextVersion(tt.release, tt.changes, tt.enforceV0, tt.bumpPatchOnNoChange)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
