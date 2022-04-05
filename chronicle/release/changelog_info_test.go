package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_getChangelogStartingRelease(t *testing.T) {
	tests := []struct {
		name     string
		summer   Summarizer
		sinceTag string
		want     *Release
		wantErr  require.ErrorAssertionFunc
	}{
		{
			name:     "use the last release when no since-tag is provided",
			sinceTag: "",
			summer: MockSummarizer{
				MockLastRelease: "v0.1.0",
			},
			want: &Release{
				Version: "v0.1.0",
			},
		},
		{
			name:     "error when fallback to last release does not exist",
			sinceTag: "",
			summer: MockSummarizer{
				MockLastRelease: "",
			},
			wantErr: require.Error,
		},
		{
			name:     "use given release (which exists)",
			sinceTag: "v0.1.0",
			summer: MockSummarizer{
				MockRelease: "v0.1.0",
			},
			want: &Release{
				Version: "v0.1.0",
			},
		},
		{
			name:     "use given release (which does not exist)",
			sinceTag: "v0.1.0",
			summer:   MockSummarizer{},
			wantErr:  require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			got, err := getChangelogStartingRelease(tt.summer, tt.sinceTag)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_changelogChanges(t *testing.T) {
	tests := []struct {
		name                string
		startReleaseVersion string
		summer              Summarizer
		config              ChangelogInfoConfig
		endReleaseVersion   string
		endReleaseDisplay   string
		wantErr             require.ErrorAssertionFunc
	}{
		{
			name:                "no end release tag discovered - speculate",
			startReleaseVersion: "v0.1.0",
			summer:              MockSummarizer{},
			config: ChangelogInfoConfig{
				VersionSpeculator: MockVersionSpeculator{
					MockNextIdealVersion:  "v0.2.0",
					MockNextUniqueVersion: "v0.2.0",
				},
			},
			endReleaseVersion: "v0.2.0",
			endReleaseDisplay: "v0.2.0",
		},
		{
			name:                "no end release tag discovered - speculate unique version",
			startReleaseVersion: "v0.1.0",
			summer:              MockSummarizer{},
			config: ChangelogInfoConfig{
				VersionSpeculator: MockVersionSpeculator{
					MockNextIdealVersion:  "v0.2.0",
					MockNextUniqueVersion: "v0.2.1",
				},
			},
			endReleaseVersion: "v0.2.1",
			endReleaseDisplay: "v0.2.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}
			endReleaseVersion, _, err := changelogChanges(tt.startReleaseVersion, tt.summer, tt.config)
			tt.wantErr(t, err)

			assert.Equal(t, tt.endReleaseVersion, endReleaseVersion)
		})
	}
}
