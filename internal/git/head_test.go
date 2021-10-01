package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeadTag(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		expects string
	}{
		{
			name:    "go case",
			path:    "test-fixtures/tagged-repo",
			expects: "v0.1.0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := HeadTagOrCommit(test.path)
			assert.NoError(t, err)
			assert.Equal(t, test.expects, actual)
		})
	}
}
