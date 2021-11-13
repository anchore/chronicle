package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeadTagOrCommit(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expects       string
		expectsLength int
	}{
		{
			name:    "head has tag",
			path:    "test-fixtures/repos/tagged-repo",
			expects: "v0.1.0",
		},
		{
			name: "head has no tag",
			path: "test-fixtures/repos/commit-in-repo",
			// since we don't commit the exact fixture, we don't know what the value will be (but the length
			// of a commit string is fixed and is a good proxy here)
			expectsLength: 40,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := HeadTagOrCommit(test.path)
			assert.NoError(t, err)
			if test.expects != "" {
				assert.Equal(t, test.expects, actual)
			}
			if test.expectsLength != 0 {
				assert.Len(t, actual, test.expectsLength)
			}
		})
	}
}

func TestHeadTag(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		expects string
	}{
		{
			name:    "head has tag",
			path:    "test-fixtures/repos/tagged-repo",
			expects: "v0.1.0",
		},
		{
			name: "head has no tag",
			path: "test-fixtures/repos/commit-in-repo",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := HeadTag(test.path)
			assert.NoError(t, err)
			assert.Equal(t, test.expects, actual)
		})
	}
}

func TestHeadCommit(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expects       string
		expectsLength int
	}{
		{
			name: "head has tag",
			path: "test-fixtures/repos/tagged-repo",
			// since we don't commit the exact fixture, we don't know what the value will be (but the length
			// of a commit string is fixed and is a good proxy here)
			expectsLength: 40,
		},
		{
			name: "head has no tag",
			path: "test-fixtures/repos/commit-in-repo",
			// since we don't commit the exact fixture, we don't know what the value will be (but the length
			// of a commit string is fixed and is a good proxy here)
			expectsLength: 40,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, err := HeadCommit(test.path)
			assert.NoError(t, err)
			if test.expects != "" {
				assert.Equal(t, test.expects, actual)
			}
			if test.expectsLength != 0 {
				assert.Len(t, actual, test.expectsLength)
			}
		})
	}
}
