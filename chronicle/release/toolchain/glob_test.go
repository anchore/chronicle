package toolchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name:    "doublestar matches root file",
			pattern: "**/go.mod",
			path:    "go.mod",
			want:    true,
		},
		{
			name:    "doublestar matches nested file",
			pattern: "**/go.mod",
			path:    "tools/ci/go.mod",
			want:    true,
		},
		{
			name:    "doublestar does not match different basename",
			pattern: "**/go.mod",
			path:    "go.sum",
			want:    false,
		},
		{
			name:    "exact pattern matches only root",
			pattern: "go.mod",
			path:    "go.mod",
			want:    true,
		},
		{
			name:    "exact pattern does not match nested",
			pattern: "go.mod",
			path:    "sub/go.mod",
			want:    false,
		},
		{
			name:    "leading ./ is ignored on pattern and path",
			pattern: "./go.mod",
			path:    "./go.mod",
			want:    true,
		},
		{
			name:    "explicit nested path matches",
			pattern: "tools/go.mod",
			path:    "tools/go.mod",
			want:    true,
		},
		{
			name:    "ignore vendor subtree",
			pattern: "**/vendor/**",
			path:    "vendor/foo/go.mod",
			want:    true,
		},
		{
			name:    "ignore vendor subtree does not match unrelated path",
			pattern: "**/vendor/**",
			path:    "internal/foo/go.mod",
			want:    false,
		},
		{
			name:    "single star within a segment",
			pattern: "cmd/*/go.mod",
			path:    "cmd/app/go.mod",
			want:    true,
		},
		{
			name:    "single star does not cross segment boundary",
			pattern: "cmd/*/go.mod",
			path:    "cmd/app/nested/go.mod",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, globMatch(tt.pattern, tt.path))
		})
	}
}
