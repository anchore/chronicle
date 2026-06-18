package options

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// TestDetectionTableMatchesRegistry guards against drift between the generated
// detection table and the dependency.Ecosystem registry: every generated entry
// must resolve to a known ecosystem (so detection emits a canonical selector and
// nothing is silently dropped). If a syft bump introduces a new declared-language
// ecosystem, either add it to the registry or accept that genecosystems skips it.
func TestDetectionTableMatchesRegistry(t *testing.T) {
	table, err := detectionTable()
	require.NoError(t, err)
	require.NotEmpty(t, table)

	for _, e := range table {
		_, ok := dependency.ParseEcosystem(e.Ecosystem)
		require.Truef(t, ok, "generated ecosystem %q is not in the dependency.Ecosystem registry", e.Ecosystem)
		require.NotEmptyf(t, e.Globs, "generated ecosystem %q has no globs", e.Ecosystem)
	}
}

func TestDetectEcosystems(t *testing.T) {
	tests := []struct {
		name    string
		files   []string
		want    []string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:  "go via go.mod",
			files: []string{"go.mod", "go.sum", "main.go"},
			want:  []string{"go"},
		},
		{
			// syft's declared catalogers key off lockfiles/manifests, so detection
			// matches what syft would actually catalog: a Cargo.lock (not Cargo.toml),
			// a package-lock.json (not package.json), etc. Results come back in
			// canonical registry order (go, javascript, python, ..., rust).
			name:  "multiple ecosystems in canonical order",
			files: []string{"Cargo.lock", "go.mod", "package-lock.json", "requirements.txt"},
			want:  []string{"go", "javascript", "python", "rust"},
		},
		{
			name:  "ruby matched by gemspec suffix glob",
			files: []string{"foo.gemspec"},
			want:  []string{"ruby"},
		},
		{
			name:  "python matched by requirements glob",
			files: []string{"dev-requirements.txt"},
			want:  []string{"python"},
		},
		{
			// package.json alone is not a declared-cataloger marker (syft keys
			// JavaScript off lockfiles), so it must not enable javascript.
			name:  "package.json alone does not detect javascript",
			files: []string{"package.json"},
			want:  nil,
		},
		{
			name:  "no recognized markers",
			files: []string{"README.md", "LICENSE"},
			want:  nil,
		},
		{
			name:  "empty dir",
			files: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			root := t.TempDir()
			for _, f := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte(""), 0600))
			}

			got, err := detectEcosystems(root)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected ecosystems (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDetectEcosystems_unreadableRoot(t *testing.T) {
	_, err := detectEcosystems(filepath.Join(t.TempDir(), "does-not-exist"))
	require.Error(t, err)
}

func TestDependencies_ResolveEcosystems(t *testing.T) {
	tests := []struct {
		name        string
		ecosystems  []string
		files       []string // markers written to the temp root
		want        []string
		wantHadAuto bool
		wantEnabled bool
		wantErr     require.ErrorAssertionFunc
	}{
		{
			name:        "auto expands to detected",
			ecosystems:  []string{"auto"},
			files:       []string{"go.mod"},
			want:        []string{"go"},
			wantHadAuto: true,
			wantEnabled: true,
		},
		{
			name:        "auto detects nothing leaves feature off",
			ecosystems:  []string{"auto"},
			files:       []string{"README.md"},
			want:        nil,
			wantHadAuto: true,
			wantEnabled: false,
		},
		{
			name:        "auto deduped against explicit",
			ecosystems:  []string{"auto", "go"},
			files:       []string{"go.mod"},
			want:        []string{"go"},
			wantHadAuto: true,
			wantEnabled: true,
		},
		{
			name:        "auto detected first then explicit preserved",
			ecosystems:  []string{"auto", "ruby"},
			files:       []string{"go.mod"},
			want:        []string{"go", "ruby"},
			wantHadAuto: true,
			wantEnabled: true,
		},
		{
			name:        "auto is case-insensitive",
			ecosystems:  []string{"AUTO"},
			files:       []string{"go.mod"},
			want:        []string{"go"},
			wantHadAuto: true,
			wantEnabled: true,
		},
		{
			name:        "no sentinel passthrough",
			ecosystems:  []string{"go", "python"},
			want:        []string{"go", "python"},
			wantHadAuto: false,
			wantEnabled: true,
		},
		{
			name:        "none disables",
			ecosystems:  []string{"none"},
			want:        nil,
			wantHadAuto: false,
			wantEnabled: false,
		},
		{
			name:        "none wins over auto",
			ecosystems:  []string{"auto", "none"},
			files:       []string{"go.mod"},
			want:        nil,
			wantHadAuto: false,
			wantEnabled: false,
		},
		{
			name:        "none wins over explicit",
			ecosystems:  []string{"go", "none"},
			want:        nil,
			wantHadAuto: false,
			wantEnabled: false,
		},
		{
			name:        "none is case-insensitive",
			ecosystems:  []string{"NONE", "go"},
			want:        nil,
			wantHadAuto: false,
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			root := t.TempDir()
			for _, f := range tt.files {
				require.NoError(t, os.WriteFile(filepath.Join(root, f), []byte(""), 0600))
			}

			c := Dependencies{Ecosystems: tt.ecosystems}
			got, hadAuto, err := c.ResolveEcosystems(root)
			tt.wantErr(t, err)
			if err != nil {
				return
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected resolved ecosystems (-want +got):\n%s", diff)
			}
			require.Equal(t, tt.wantHadAuto, hadAuto)
			require.Equal(t, tt.wantEnabled, c.Enabled())
		})
	}
}
