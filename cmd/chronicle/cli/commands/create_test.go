package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckSpeculationRange(t *testing.T) {
	tests := []struct {
		name    string
		config  createConfig
		wantErr string
	}{
		{
			name:   "no speculation and no until-tag",
			config: createConfig{},
		},
		{
			name:   "until-tag without speculation",
			config: createConfig{UntilTag: "v0.13.0"},
		},
		{
			name:   "speculation without until-tag",
			config: createConfig{SpeculateNextVersion: true},
		},
		{
			name:    "speculation with until-tag",
			config:  createConfig{SpeculateNextVersion: true, UntilTag: "v0.13.0"},
			wantErr: `--speculate-next-version cannot be used with --until-tag: the changelog range already ends at "v0.13.0", which pins the version`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSpeculationRange(&tt.config)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
