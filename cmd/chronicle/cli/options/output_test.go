package options

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/output"
)

func TestOutput_Specs(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Output
		want    []output.Spec
		wantErr require.ErrorAssertionFunc
	}{
		{
			name: "default constructor → md to stdout",
			cfg:  DefaultOutput(),
			want: []output.Spec{{Name: "md"}},
		},
		{
			name: "single output passthrough",
			cfg:  Output{Outputs: []string{"json"}},
			want: []output.Spec{{Name: "json"}},
		},
		{
			name: "deprecated version-file appended",
			cfg:  Output{Outputs: []string{"md"}, VersionFile: "VERSION"},
			want: []output.Spec{{Name: "md"}, {Name: "version", Path: "VERSION"}},
		},
		{
			name: "version-file alone produces just the version spec",
			cfg:  Output{VersionFile: "VERSION"},
			want: []output.Spec{{Name: "version", Path: "VERSION"}},
		},
		{
			name: "multiple outputs with files",
			cfg: Output{Outputs: []string{
				"md=CHANGELOG.md",
				"version=VERSION",
				"json",
			}},
			want: []output.Spec{
				{Name: "md", Path: "CHANGELOG.md"},
				{Name: "version", Path: "VERSION"},
				{Name: "json"},
			},
		},
		{
			name:    "malformed spec",
			cfg:     Output{Outputs: []string{"md="}},
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := tt.cfg.Specs()
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("specs mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDefaultOutput_Encoders(t *testing.T) {
	o := DefaultOutput()
	require.ElementsMatch(t, []string{"md", "json", "version"}, o.Available.Names())
}

// TestOutput_Writer_EndToEnd is the seam between the cmd layer and the output
// package: it exercises DefaultOutput → Writer() → real encoders → real files.
// Catches construction-time wiring bugs that the unit-level tests (which
// substitute fake encoders or empty descriptions) wouldn't surface.
func TestOutput_Writer_EndToEnd(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "CHANGELOG.md")
	versionPath := filepath.Join(dir, "VERSION")

	o := DefaultOutput()
	o.Outputs = []string{
		"md=" + mdPath,
		"version=" + versionPath,
	}

	w, err := o.Writer()
	require.NoError(t, err)

	desc := release.Description{
		Release:       release.Release{Version: "v1.2.3"},
		VCSChangesURL: "https://example.com/compare/v1.2.2...v1.2.3",
	}

	require.NoError(t, w.Write("My Title", desc))
	require.NoError(t, w.Close())

	md, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	require.Contains(t, string(md), "# My Title")
	require.Contains(t, string(md), "https://example.com/compare/v1.2.2...v1.2.3")

	ver, err := os.ReadFile(versionPath)
	require.NoError(t, err)
	require.Equal(t, "v1.2.3\n", string(ver))
}

// TestOutput_Writer_EmptyOutputsErrors pins the contract that an explicit empty
// Outputs (e.g. `output: []` in yaml) is an error rather than silently
// re-defaulting to markdown. The default value lives in DefaultOutput, not in
// runtime fallback logic.
func TestOutput_Writer_EmptyOutputsErrors(t *testing.T) {
	o := DefaultOutput()
	o.Outputs = nil

	_, err := o.Writer()
	require.Error(t, err)
	require.Contains(t, err.Error(), "no output specs")
}
