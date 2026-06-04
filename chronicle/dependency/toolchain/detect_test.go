package toolchain

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/internal/git"
)

func goMod(version string) []byte {
	return []byte("module example.com/foo\n\ngo " + version + "\n")
}

func TestDetect(t *testing.T) {
	// a config that enables detection for all known ecosystems with default discovery globs and the
	// default ignore set; individual tests tweak copies of this.
	baseCfg := func() Config {
		return Config{
			Enabled: true,
			Ignore:  []string{"**/vendor/**", "**/testdata/**"},
		}
	}

	tests := []struct {
		name       string
		cfg        Config
		since      string
		until      string
		sinceFiles []git.FileBlob
		untilFiles []git.FileBlob
		want       *release.ToolchainData
		wantErr    require.ErrorAssertionFunc
	}{
		{
			name:  "disabled is a no-op",
			cfg:   Config{Enabled: false},
			since: "v1",
			until: "v2",
			want:  nil,
		},
		{
			name:  "no since ref skips detection",
			cfg:   baseCfg(),
			since: "",
			until: "v2",
			want:  nil,
		},
		{
			name:       "single go.mod bump",
			cfg:        baseCfg(),
			since:      "v1",
			until:      "v2",
			sinceFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			untilFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.23")}},
			want: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.21", To: "1.23", Direction: release.ToolchainUpgrade},
				},
			},
		},
		{
			name:       "downgrade is flagged",
			cfg:        baseCfg(),
			since:      "v1",
			until:      "v2",
			sinceFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.23")}},
			untilFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			want: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.23", To: "1.21", Direction: release.ToolchainDowngrade},
				},
			},
		},
		{
			name:       "no change yields nil",
			cfg:        baseCfg(),
			since:      "v1",
			until:      "v2",
			sinceFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			untilFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			want:       nil,
		},
		{
			name:  "multi-module identical transition is not a conflict",
			cfg:   baseCfg(),
			since: "v1",
			until: "v2",
			sinceFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.21")},
				{Path: "tools/go.mod", Content: goMod("1.21")},
			},
			untilFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.23")},
				{Path: "tools/go.mod", Content: goMod("1.23")},
			},
			want: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.21", To: "1.23", Direction: release.ToolchainUpgrade},
					{Tool: "go", Source: "go directive", File: "tools/go.mod", From: "1.21", To: "1.23", Direction: release.ToolchainUpgrade},
				},
			},
		},
		{
			name:  "multi-module divergence warns",
			cfg:   baseCfg(),
			since: "v1",
			until: "v2",
			sinceFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.20")},
				{Path: "tools/go.mod", Content: goMod("1.20")},
			},
			untilFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.23")},
				{Path: "tools/go.mod", Content: goMod("1.22")},
			},
			want: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.20", To: "1.23", Direction: release.ToolchainUpgrade},
					{Tool: "go", Source: "go directive", File: "tools/go.mod", From: "1.20", To: "1.22", Direction: release.ToolchainUpgrade},
				},
				Warnings: []release.ToolchainWarning{
					{
						Tool:    "go",
						Message: "Go sources disagree on the resulting version: 1.23 (go.mod); 1.22 (tools/go.mod)",
						Files:   []string{"go.mod", "tools/go.mod"},
					},
				},
			},
		},
		{
			name:  "ignored paths are not discovered",
			cfg:   baseCfg(),
			since: "v1",
			until: "v2",
			sinceFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.21")},
				{Path: "vendor/dep/go.mod", Content: goMod("1.10")},
			},
			untilFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.21")},
				{Path: "vendor/dep/go.mod", Content: goMod("1.18")}, // changed, but ignored
			},
			want: nil,
		},
		{
			name:       "explicit ecosystem selection runs only requested",
			cfg:        Config{Enabled: true, Ecosystems: []dependency.Ecosystem{dependency.EcosystemGo}},
			since:      "v1",
			until:      "v2",
			sinceFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			untilFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.22")}},
			want: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.21", To: "1.22", Direction: release.ToolchainUpgrade},
				},
			},
		},
		{
			name:       "unknown ecosystem selection yields nothing",
			cfg:        Config{Enabled: true, Ecosystems: []dependency.Ecosystem{"cobol"}},
			since:      "v1",
			until:      "v2",
			sinceFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			untilFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.22")}},
			want:       nil,
		},
		{
			name:       "unparseable file degrades gracefully",
			cfg:        baseCfg(),
			since:      "v1",
			until:      "v2",
			sinceFiles: []git.FileBlob{{Path: "go.mod", Content: goMod("1.21")}},
			untilFiles: []git.FileBlob{{Path: "go.mod", Content: []byte("module example.com/foo\n\ngo not-a-version\n")}},
			want:       nil,
		},
		{
			name:  "explicit path override narrows discovery to root",
			cfg:   Config{Enabled: true, Paths: map[dependency.Ecosystem][]string{dependency.EcosystemGo: {"go.mod"}}},
			since: "v1",
			until: "v2",
			sinceFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.21")},
				{Path: "tools/go.mod", Content: goMod("1.20")},
			},
			untilFiles: []git.FileBlob{
				{Path: "go.mod", Content: goMod("1.21")},       // unchanged
				{Path: "tools/go.mod", Content: goMod("1.23")}, // changed, but not discovered
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			gitter := git.MockInterface{
				MockFilesAtRef: map[string][]git.FileBlob{
					tt.since: tt.sinceFiles,
					tt.until: tt.untilFiles,
				},
			}

			got, err := Detect(gitter, tt.cfg, tt.since, tt.until)
			tt.wantErr(t, err)

			if err != nil {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
