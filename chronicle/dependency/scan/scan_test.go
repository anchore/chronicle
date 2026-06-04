package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestScanner_Catalog_Exclude exercises the syft exclude wiring end-to-end: a
// scan tree with a root manifest and two nested ones, scanned under various
// exclude patterns. It deliberately passes the raw t.TempDir() (which on macOS
// lives behind the /var → /private/var symlink) so it also covers Catalog's
// symlink canonicalization — without which the excludes silently match nothing.
func TestScanner_Catalog_Exclude(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, filepath.Join(root, "requirements.txt"), "rootdep==1.0.0")
	writeManifest(t, filepath.Join(root, "vendor", "requirements.txt"), "vendordep==2.0.0")
	writeManifest(t, filepath.Join(root, "pkg", "testdata", "requirements.txt"), "testdep==3.0.0")

	tests := []struct {
		name     string
		exclude  []string
		wantPkgs []string
	}{
		{
			name:     "no excludes scans everything",
			exclude:  nil,
			wantPkgs: []string{"rootdep", "testdep", "vendordep"},
		},
		{
			name:     "exclude a top-level dir",
			exclude:  []string{"./vendor"},
			wantPkgs: []string{"rootdep", "testdep"},
		},
		{
			name:     "exclude a nested dir by glob",
			exclude:  []string{"**/testdata"},
			wantPkgs: []string{"rootdep", "vendordep"},
		},
		{
			name:     "multiple excludes",
			exclude:  []string{"./vendor", "**/testdata"},
			wantPkgs: []string{"rootdep"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat, err := NewScanner([]string{"python"}, tt.exclude).
				Catalog(context.Background(), root, SourceInfo{Name: "test", Version: "v0"}, Hooks{})
			require.NoError(t, err)

			got := make([]string, 0, len(cat.Packages))
			for _, p := range cat.Packages {
				got = append(got, p.Name)
			}
			sort.Strings(got)
			require.Equal(t, tt.wantPkgs, got)
		})
	}
}

func writeManifest(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content+"\n"), 0o644))
}
