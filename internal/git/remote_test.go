package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteUrl(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		expects string
	}{
		{
			name:    "go case",
			path:    "test-fixtures/repos/remote-repo",
			expects: "git@github.com:wagoodman/count-goober.git",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := RemoteURL(test.path)
			require.NoError(t, err)
			assert.Equal(t, test.expects, actual)
		})
	}
}
