package release

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestToolchainData_DisplayLines(t *testing.T) {
	tests := []struct {
		name string
		data *ToolchainData
		want []ToolchainDisplay
	}{
		{
			name: "nil data",
			data: nil,
			want: nil,
		},
		{
			name: "single update has no file disambiguation",
			data: &ToolchainData{
				Updates: []ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.21", To: "1.23", Direction: ToolchainUpgrade},
				},
			},
			want: []ToolchainDisplay{
				{Label: "Go", From: "1.21", To: "1.23", Direction: ToolchainUpgrade},
			},
		},
		{
			name: "identical transition across modules collapses to one line",
			data: &ToolchainData{
				Updates: []ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.21", To: "1.23", Direction: ToolchainUpgrade},
					{Tool: "go", Source: "go directive", File: "tools/go.mod", From: "1.21", To: "1.23", Direction: ToolchainUpgrade},
				},
			},
			want: []ToolchainDisplay{
				{Label: "Go", From: "1.21", To: "1.23", Direction: ToolchainUpgrade},
			},
		},
		{
			name: "downgrade direction is carried through",
			data: &ToolchainData{
				Updates: []ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.23", To: "1.21", Direction: ToolchainDowngrade},
				},
			},
			want: []ToolchainDisplay{
				{Label: "Go", From: "1.23", To: "1.21", Direction: ToolchainDowngrade},
			},
		},
		{
			name: "divergent transitions disambiguate with files",
			data: &ToolchainData{
				Updates: []ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.20", To: "1.23", Direction: ToolchainUpgrade},
					{Tool: "go", Source: "go directive", File: "tools/go.mod", From: "1.20", To: "1.22", Direction: ToolchainUpgrade},
				},
			},
			want: []ToolchainDisplay{
				{Label: "Go", From: "1.20", To: "1.23", Direction: ToolchainUpgrade, Files: []string{"go.mod"}},
				{Label: "Go", From: "1.20", To: "1.22", Direction: ToolchainUpgrade, Files: []string{"tools/go.mod"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.DisplayLines()
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
