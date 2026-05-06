package output

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestParseSpec(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Spec
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "bare name → stdout",
			input: "md",
			want:  Spec{Name: "md"},
		},
		{
			name:  "name with explicit stdout dash",
			input: "md=-",
			want:  Spec{Name: "md"},
		},
		{
			name:  "name with file path",
			input: "md=CHANGELOG.md",
			want:  Spec{Name: "md", Path: "CHANGELOG.md"},
		},
		{
			name:  "path with equals sign in it",
			input: "md=./out=foo.md",
			want:  Spec{Name: "md", Path: "./out=foo.md"},
		},
		{
			name:  "leading/trailing whitespace on name preserved trimmed",
			input: "  md  ",
			want:  Spec{Name: "md"},
		},
		{
			name:    "empty",
			input:   "",
			wantErr: require.Error,
		},
		{
			name:    "only whitespace",
			input:   "   ",
			wantErr: require.Error,
		},
		{
			name:    "missing name",
			input:   "=path",
			wantErr: require.Error,
		},
		{
			name:    "empty path after =",
			input:   "md=",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			got, err := ParseSpec(tt.input)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Spec mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		specs   []Spec
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "single stdout entry",
			specs: []Spec{{Name: "md"}},
		},
		{
			name:  "stdout plus file is fine",
			specs: []Spec{{Name: "md"}, {Name: "version", Path: "VERSION"}},
		},
		{
			name:  "two file entries with different paths",
			specs: []Spec{{Name: "md", Path: "a.md"}, {Name: "json", Path: "a.json"}},
		},
		{
			name:    "no specs",
			specs:   nil,
			wantErr: require.Error,
		},
		{
			name:    "two stdout entries",
			specs:   []Spec{{Name: "md"}, {Name: "json"}},
			wantErr: require.Error,
		},
		{
			name:    "same path twice",
			specs:   []Spec{{Name: "md", Path: "out.md"}, {Name: "json", Path: "out.md"}},
			wantErr: require.Error,
		},
		// note: unknown encoder names are no longer Validate's job; that check
		// belongs to New, where the caller-supplied Encoders set is in scope.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			err := Validate(tt.specs)
			tt.wantErr(t, err)
		})
	}
}
