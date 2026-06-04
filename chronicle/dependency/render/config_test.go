package render

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
)

func TestConfig_ResolveDisplay(t *testing.T) {
	tests := []struct {
		name             string
		modes            []Mode
		supportsCollapse bool
		want             Mode
	}{
		{
			name:             "collapsed supported uses collapsed",
			modes:            []Mode{ModeCollapsed, ModeList},
			supportsCollapse: true,
			want:             ModeCollapsed,
		},
		{
			name:             "collapsed unsupported falls through to list",
			modes:            []Mode{ModeCollapsed, ModeList},
			supportsCollapse: false,
			want:             ModeList,
		},
		{
			name:             "collapsed unsupported falls through to summary",
			modes:            []Mode{ModeCollapsed, ModeSummary},
			supportsCollapse: false,
			want:             ModeSummary,
		},
		{
			name:             "bare collapsed unsupported degrades to list",
			modes:            []Mode{ModeCollapsed},
			supportsCollapse: false,
			want:             ModeList,
		},
		{
			name:             "hide wins regardless of support",
			modes:            []Mode{ModeHide},
			supportsCollapse: true,
			want:             ModeHide,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := Config{Actions: map[dependency.ChangeKind][]Mode{dependency.Updated: tt.modes}}
			require.Equal(t, tt.want, rc.ResolveDisplay(dependency.Updated, tt.supportsCollapse))
		})
	}
}

func TestConfig_ResolveDisplay_OnlyVulnerableUpgradesSummary(t *testing.T) {
	// under OnlyVulnerable an explicitly-configured summary becomes a list so the
	// vulnerable packages are actually shown (the rollup totals stay in
	// SummaryLine). This is the md-pretty case: can't collapse, so a
	// collapsed,summary kind falls through to summary and would otherwise render
	// as an empty header.
	rc := &Config{OnlyVulnerable: true, Actions: map[dependency.ChangeKind][]Mode{
		dependency.Added:   {ModeSummary},
		dependency.Removed: {ModeCollapsed, ModeSummary},
	}}
	require.Equal(t, ModeList, rc.ResolveDisplay(dependency.Added, false))   // summary upgraded to list
	require.Equal(t, ModeList, rc.ResolveDisplay(dependency.Removed, false)) // collapse unsupported → summary → list

	// the upgrade only fires when collapse is unavailable; collapsed still wins
	// where supported.
	require.Equal(t, ModeCollapsed, rc.ResolveDisplay(dependency.Removed, true))

	// hide still wins regardless.
	hide := &Config{OnlyVulnerable: true, Actions: map[dependency.ChangeKind][]Mode{dependency.Added: {ModeHide}}}
	require.Equal(t, ModeHide, hide.ResolveDisplay(dependency.Added, false))
}

func TestConfig_ResolveDisplay_DefaultsWhenEmpty(t *testing.T) {
	var rc *Config // nil config falls back to defaults
	// default for Updated is collapsed,list: collapses where supported...
	require.Equal(t, ModeCollapsed, rc.ResolveDisplay(dependency.Updated, true))
	// ...and enumerates where it can't collapse.
	require.Equal(t, ModeList, rc.ResolveDisplay(dependency.Updated, false))
	// default for Added is collapsed,list: enumerates where it can't collapse.
	require.Equal(t, ModeCollapsed, rc.ResolveDisplay(dependency.Added, true))
	require.Equal(t, ModeList, rc.ResolveDisplay(dependency.Added, false))
}

func TestConfig_PackageCountLabel(t *testing.T) {
	plain := &Config{}
	require.Equal(t, "11 packages", plain.PackageCountLabel(11))
	require.Equal(t, "1 package", plain.PackageCountLabel(1))

	vuln := &Config{OnlyVulnerable: true}
	require.Equal(t, "11 packages with vulnerability changes", vuln.PackageCountLabel(11))
}

func TestConfig_VisibleChanges(t *testing.T) {
	changes := []dependency.PackageChange{
		{Name: "a", Kind: dependency.Updated, Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-1"}}}},
		{Name: "b", Kind: dependency.Updated, Vuln: &dependency.VulnDelta{}}, // annotated, no impact
		{Name: "c", Kind: dependency.Added}, // unannotated
	}

	// nil / not-only-vulnerable returns the input unchanged.
	require.Equal(t, changes, (&Config{}).VisibleChanges(changes))
	require.Equal(t, changes, (*Config)(nil).VisibleChanges(changes))

	// only-vulnerable keeps just the change with impact.
	got := (&Config{OnlyVulnerable: true}).VisibleChanges(changes)
	require.Len(t, got, 1)
	require.Equal(t, "a", got[0].Name)
}

func TestParseModes(t *testing.T) {
	require.Equal(t, []Mode{ModeCollapsed, ModeList}, ParseModes("collapsed,list"))
	// aliases: collapse -> collapsed, enumerate -> list; whitespace trimmed.
	require.Equal(t, []Mode{ModeCollapsed, ModeList}, ParseModes(" collapse , enumerate "))
	// unknown entries are dropped.
	require.Equal(t, []Mode{ModeSummary}, ParseModes("bogus,summary"))
	require.Nil(t, ParseModes("nonsense"))
}
