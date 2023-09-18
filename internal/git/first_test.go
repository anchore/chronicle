package git

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstCommit(t *testing.T) {

	tests := []struct {
		name     string
		repoPath string
		want     string
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "gocase",
			repoPath: "test-fixtures/repos/tag-range-repo",
			want:     gitFirstCommit(t, "test-fixtures/repos/tag-range-repo"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = assert.NoError
			}
			got, err := FirstCommit(tt.repoPath)
			if !tt.wantErr(t, err, fmt.Sprintf("FirstCommit(%v)", tt.repoPath)) {
				return
			}
			assert.Equalf(t, tt.want, got, "FirstCommit(%v)", tt.repoPath)
		})
	}
}

func gitFirstCommit(t *testing.T, path string) string {
	t.Helper()

	cmd := exec.Command("git", "--no-pager", "log", "--reverse", `--pretty=format:%H`)
	cmd.Dir = path
	output, err := cmd.Output()
	require.NoError(t, err)

	rows := strings.Split(strings.TrimSpace(string(output)), "\n")
	require.NotEmpty(t, rows)
	return rows[0]
}
