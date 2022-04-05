package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/internal/git"
)

func TestFindChangelogEndTag(t *testing.T) {
	tests := []struct {
		name    string
		summer  release.Summarizer
		gitter  git.Interface
		want    string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:   "no release for existing tag at head should return head tag",
			summer: release.MockSummarizer{},
			gitter: git.MockInterface{
				MockHeadTag: "v0.1.0",
			},
			want: "v0.1.0",
		},
		{
			name: "release for existing tag at head should return no tag",
			gitter: git.MockInterface{
				MockHeadTag: "v0.1.0",
			},
			summer: release.MockSummarizer{
				MockRelease: "v0.1.0",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := FindChangelogEndTag(tt.summer, tt.gitter)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
