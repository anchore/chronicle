package toolchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/internal/git"
)

func TestDirtySourceFiles(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		dirty []string
		want  []string
	}{
		{
			name:  "disabled returns nothing",
			cfg:   Config{Enabled: false},
			dirty: []string{"go.mod"},
			want:  nil,
		},
		{
			name:  "matches a dirty toolchain source file",
			cfg:   Config{Enabled: true},
			dirty: []string{"go.mod", "README.md", "main.go"},
			want:  []string{"go.mod"},
		},
		{
			name:  "matches nested module manifests",
			cfg:   Config{Enabled: true},
			dirty: []string{"tools/go.mod", "internal/foo.go"},
			want:  []string{"tools/go.mod"},
		},
		{
			name:  "ignored paths are not flagged",
			cfg:   Config{Enabled: true, Ignore: []string{"**/vendor/**"}},
			dirty: []string{"vendor/dep/go.mod"},
			want:  nil,
		},
		{
			name:  "clean tree returns nothing",
			cfg:   Config{Enabled: true},
			dirty: nil,
			want:  nil,
		},
		{
			name:  "path override narrows matching",
			cfg:   Config{Enabled: true, Paths: map[dependency.Ecosystem][]string{dependency.EcosystemGo: {"go.mod"}}},
			dirty: []string{"tools/go.mod"},
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gitter := git.MockInterface{MockDirtyPaths: tt.dirty}

			got, err := DirtySourceFiles(gitter, tt.cfg)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
