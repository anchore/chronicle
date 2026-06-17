package dependency

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func vuln(id, severity string) Vulnerability {
	return Vulnerability{ID: id, Severity: severity}
}

func vulnMap(entries ...struct {
	key   PackageKey
	vulns []Vulnerability
}) map[PackageKey][]Vulnerability {
	m := make(map[PackageKey][]Vulnerability)
	for _, e := range entries {
		m[e.key] = e.vulns
	}
	return m
}

func goKey(name string) PackageKey { return PackageKey{Type: "go-module", Name: name} }

func TestAnnotate(t *testing.T) {
	tests := []struct {
		name           string
		changes        []PackageChange
		since          Scan
		until          Scan
		cfg            annotateConfig
		wantChanges    []PackageChange
		wantRemediated int
		wantIntroduced int
	}{
		{
			// updated package: vulns that disappeared are Remediated; new ones are Introduced
			name: "updated package - remediated and introduced set difference",
			changes: []PackageChange{
				{Name: "lib", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
			since: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-001", "high"), vuln("CVE-002", "medium")}}),
			},
			until: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-002", "medium"), vuln("CVE-003", "low")}}),
			},
			cfg: annotateConfig{},
			wantChanges: []PackageChange{
				{
					Name: "lib", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated,
					Vuln: &VulnDelta{
						Remediated: []Vulnerability{vuln("CVE-001", "high")},
						Introduced: []Vulnerability{vuln("CVE-003", "low")},
					},
				},
			},
			wantRemediated: 1,
			wantIntroduced: 1,
		},
		{
			// removed package: all since vulns are remediated
			name: "removed package - all since vulns become remediated",
			changes: []PackageChange{
				{Name: "old", Type: "go-module", FromVersion: "1", ToVersion: "", Kind: Removed},
			},
			since: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("old"), []Vulnerability{vuln("CVE-A", "critical"), vuln("CVE-B", "low")}}),
			},
			until: Scan{Vulns: nil},
			cfg:   annotateConfig{},
			wantChanges: []PackageChange{
				{
					Name: "old", Type: "go-module", FromVersion: "1", ToVersion: "", Kind: Removed,
					Vuln: &VulnDelta{
						Remediated: []Vulnerability{vuln("CVE-A", "critical"), vuln("CVE-B", "low")},
					},
				},
			},
			wantRemediated: 2,
			wantIntroduced: 0,
		},
		{
			// added package: all until vulns are introduced
			name: "added package - all until vulns become introduced",
			changes: []PackageChange{
				{Name: "new", Type: "go-module", FromVersion: "", ToVersion: "3", Kind: Added},
			},
			since: Scan{Vulns: nil},
			until: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("new"), []Vulnerability{vuln("GHSA-XYZ", "high")}}),
			},
			cfg: annotateConfig{},
			wantChanges: []PackageChange{
				{
					Name: "new", Type: "go-module", FromVersion: "", ToVersion: "3", Kind: Added,
					Vuln: &VulnDelta{
						Introduced: []Vulnerability{vuln("GHSA-XYZ", "high")},
					},
				},
			},
			wantRemediated: 0,
			wantIntroduced: 1,
		},
		{
			// downgrade with introduced vulns — the renderer detects this as
			// Kind==Downgraded && len(Introduced)>0
			name: "downgraded package reintroduces vulns",
			changes: []PackageChange{
				{Name: "lib", Type: "go-module", FromVersion: "5", ToVersion: "2", Kind: Downgraded},
			},
			since: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-OLD", "high")}}),
			},
			until: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-OLD", "high"), vuln("CVE-NEW", "critical")}}),
			},
			cfg: annotateConfig{},
			wantChanges: []PackageChange{
				{
					Name: "lib", Type: "go-module", FromVersion: "5", ToVersion: "2", Kind: Downgraded,
					Vuln: &VulnDelta{
						Introduced: []Vulnerability{vuln("CVE-NEW", "critical")},
					},
				},
			},
			wantRemediated: 0,
			wantIntroduced: 1,
		},
		{
			// MinSeverity filters out vulns below the floor
			name: "min severity floor filters low-severity vulns",
			changes: []PackageChange{
				{Name: "lib", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
			since: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{
					vuln("CVE-HIGH", "high"),
					vuln("CVE-LOW", "low"),
					vuln("CVE-NEG", "negligible"),
				}}),
			},
			until: Scan{Vulns: nil},
			cfg:   annotateConfig{MinSeverity: "high"},
			wantChanges: []PackageChange{
				{
					Name: "lib", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated,
					Vuln: &VulnDelta{
						Remediated: []Vulnerability{vuln("CVE-HIGH", "high")},
					},
				},
			},
			wantRemediated: 1,
			wantIntroduced: 0,
		},
		{
			// unique ID deduplication: same CVE appearing in two packages counts once
			name: "unique id dedup across packages",
			changes: []PackageChange{
				{Name: "pkg-a", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
				{Name: "pkg-b", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
			since: Scan{
				Vulns: vulnMap(
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("pkg-a"), []Vulnerability{vuln("CVE-SHARED", "high"), vuln("CVE-A-ONLY", "medium")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("pkg-b"), []Vulnerability{vuln("CVE-SHARED", "high")}},
				),
			},
			until: Scan{Vulns: nil},
			cfg:   annotateConfig{},
			wantChanges: []PackageChange{
				{
					Name: "pkg-a", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated,
					Vuln: &VulnDelta{
						Remediated: []Vulnerability{vuln("CVE-SHARED", "high"), vuln("CVE-A-ONLY", "medium")},
					},
				},
				{
					Name: "pkg-b", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated,
					Vuln: &VulnDelta{
						Remediated: []Vulnerability{vuln("CVE-SHARED", "high")},
					},
				},
			},
			// CVE-SHARED appears in both packages but counts only once
			wantRemediated: 2,
			wantIntroduced: 0,
		},
		{
			// empty vuln maps produce an annotated-but-empty delta (non-nil Vuln)
			name: "no vulns produces empty annotations",
			changes: []PackageChange{
				{Name: "x", Type: "npm", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
			since: Scan{Vulns: nil},
			until: Scan{Vulns: nil},
			cfg:   annotateConfig{},
			wantChanges: []PackageChange{
				{Name: "x", Type: "npm", FromVersion: "1", ToVersion: "2", Kind: Updated, Vuln: &VulnDelta{}},
			},
			wantRemediated: 0,
			wantIntroduced: 0,
		},
		{
			// MinSeverity="" means no filtering
			name: "empty min severity passes all vulns through",
			changes: []PackageChange{
				{Name: "lib", Type: "go-module", FromVersion: "1", ToVersion: "", Kind: Removed},
			},
			since: Scan{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{
					vuln("CVE-NEG", "negligible"),
					vuln("CVE-UNK", ""),
				}}),
			},
			until: Scan{Vulns: nil},
			cfg:   annotateConfig{MinSeverity: ""},
			wantChanges: []PackageChange{
				{
					Name: "lib", Type: "go-module", FromVersion: "1", ToVersion: "", Kind: Removed,
					Vuln: &VulnDelta{
						Remediated: []Vulnerability{vuln("CVE-NEG", "negligible"), vuln("CVE-UNK", "")},
					},
				},
			},
			wantRemediated: 2,
			wantIntroduced: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// the scans being compared now live on the diff; annotate reads
			// them from there rather than as separate arguments.
			d := NewDiff(tt.changes)
			d.Since = tt.since
			d.Until = tt.until
			got := annotate(d, tt.cfg)

			if d := cmp.Diff(tt.wantChanges, got.Changes); d != "" {
				t.Errorf("annotate() changes mismatch (-want +got):\n%s", d)
			}
			require.Equal(t, tt.wantRemediated, got.RemediatedCount)
			require.Equal(t, tt.wantIntroduced, got.IntroducedCount)
		})
	}
}

func TestAnnotate_Remaining(t *testing.T) {
	goPkg := func(name, version string) Package { return Package{Name: name, Version: version, Type: "go-module"} }

	tests := []struct {
		name          string
		changes       []PackageChange
		since         Scan
		until         Scan
		cfg           annotateConfig
		wantRemaining []PackageVulns
		wantCount     int
	}{
		{
			// the core case: a vuln present at both refs is "remaining" (carried
			// over) — distinct from remediated (gone at until) and introduced (new
			// at until). It also spans an unchanged package the diff never lists.
			name: "carried-over vulns across changed and unchanged packages",
			changes: []PackageChange{
				{Name: "carry", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
			since: Scan{
				Packages: []Package{goPkg("carry", "1"), goPkg("stable", "9"), goPkg("dropped", "1")},
				Vulns: vulnMap(
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("carry"), []Vulnerability{vuln("CVE-OLD", "high"), vuln("CVE-FIXED", "low")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("stable"), []Vulnerability{vuln("CVE-STABLE", "medium")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("dropped"), []Vulnerability{vuln("CVE-GONE", "high")}},
				),
			},
			until: Scan{
				Packages: []Package{goPkg("carry", "2"), goPkg("stable", "9"), goPkg("fresh", "1")},
				Vulns: vulnMap(
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("carry"), []Vulnerability{vuln("CVE-OLD", "high"), vuln("CVE-NEW", "critical")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("stable"), []Vulnerability{vuln("CVE-STABLE", "medium")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("fresh"), []Vulnerability{vuln("CVE-INTRO", "high")}},
				),
			},
			wantRemaining: []PackageVulns{
				{Package: goPkg("carry", "2"), Vulns: []Vulnerability{vuln("CVE-OLD", "high")}},
				{Package: goPkg("stable", "9"), Vulns: []Vulnerability{vuln("CVE-STABLE", "medium")}},
			},
			wantCount: 2,
		},
		{
			// the same carried-over ID on two packages counts once.
			name:    "unique id dedup across packages",
			changes: nil,
			since: Scan{
				Packages: []Package{goPkg("a", "1"), goPkg("b", "1")},
				Vulns: vulnMap(
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("a"), []Vulnerability{vuln("CVE-SHARED", "high")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("b"), []Vulnerability{vuln("CVE-SHARED", "high")}},
				),
			},
			until: Scan{
				Packages: []Package{goPkg("a", "1"), goPkg("b", "1")},
				Vulns: vulnMap(
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("a"), []Vulnerability{vuln("CVE-SHARED", "high")}},
					struct {
						key   PackageKey
						vulns []Vulnerability
					}{goKey("b"), []Vulnerability{vuln("CVE-SHARED", "high")}},
				),
			},
			wantRemaining: []PackageVulns{
				{Package: goPkg("a", "1"), Vulns: []Vulnerability{vuln("CVE-SHARED", "high")}},
				{Package: goPkg("b", "1"), Vulns: []Vulnerability{vuln("CVE-SHARED", "high")}},
			},
			wantCount: 1,
		},
		{
			// MinSeverity filters the carried-over set just like the per-change deltas.
			name:    "min severity floor filters carried-over vulns",
			changes: nil,
			since: Scan{
				Packages: []Package{goPkg("lib", "1")},
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-HIGH", "high"), vuln("CVE-LOW", "low")}}),
			},
			until: Scan{
				Packages: []Package{goPkg("lib", "1")},
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-HIGH", "high"), vuln("CVE-LOW", "low")}}),
			},
			cfg: annotateConfig{MinSeverity: "high"},
			wantRemaining: []PackageVulns{
				{Package: goPkg("lib", "1"), Vulns: []Vulnerability{vuln("CVE-HIGH", "high")}},
			},
			wantCount: 1,
		},
		{
			// until not matched (nil Vulns) yields no remaining, not a panic.
			name:          "until not matched yields no remaining",
			changes:       nil,
			since:         Scan{Vulns: vulnMap()},
			until:         Scan{Vulns: nil},
			wantRemaining: nil,
			wantCount:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDiff(tt.changes)
			d.Since = tt.since
			d.Until = tt.until
			got := annotate(d, tt.cfg)

			if diff := cmp.Diff(tt.wantRemaining, got.Remaining); diff != "" {
				t.Errorf("annotate() remaining mismatch (-want +got):\n%s", diff)
			}
			require.Equal(t, tt.wantCount, got.RemainingCount)
		})
	}
}

func TestIntersectByID(t *testing.T) {
	tests := []struct {
		name string
		a    []Vulnerability
		b    []Vulnerability
		want []Vulnerability
	}{
		{
			name: "empty inputs",
			a:    nil,
			b:    nil,
			want: nil,
		},
		{
			name: "none shared",
			a:    []Vulnerability{vuln("CVE-1", "high")},
			b:    []Vulnerability{vuln("CVE-2", "low")},
			want: nil,
		},
		{
			name: "overlap: only shared returned, from a's records",
			a:    []Vulnerability{vuln("CVE-1", "high"), vuln("CVE-2", "low")},
			b:    []Vulnerability{vuln("CVE-2", "low"), vuln("CVE-3", "medium")},
			want: []Vulnerability{vuln("CVE-2", "low")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersectByID(tt.a, tt.b)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("intersectByID() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{input: "", want: 0},
		{input: "unknown", want: 0},
		{input: "negligible", want: 1},
		{input: "NEGLIGIBLE", want: 1},
		{input: "low", want: 2},
		{input: "LOW", want: 2},
		{input: "medium", want: 3},
		{input: "Medium", want: 3},
		{input: "high", want: 4},
		{input: "critical", want: 5},
		{input: "CRITICAL", want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := severityRank(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestValidSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "", want: true}, // empty means "no filter"
		{input: "  ", want: true},
		{input: "negligible", want: true},
		{input: "CRITICAL", want: true},
		{input: " High ", want: true},
		{input: "unknown", want: false},
		{input: "moderate", want: false}, // common typo for "medium"
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, ValidSeverity(tt.input))
		})
	}
}

func TestSetDifference(t *testing.T) {
	tests := []struct {
		name string
		a    []Vulnerability
		b    []Vulnerability
		want []Vulnerability
	}{
		{
			name: "empty inputs",
			a:    nil,
			b:    nil,
			want: nil,
		},
		{
			name: "all in a, none in b",
			a:    []Vulnerability{vuln("CVE-1", "high"), vuln("CVE-2", "low")},
			b:    nil,
			want: []Vulnerability{vuln("CVE-1", "high"), vuln("CVE-2", "low")},
		},
		{
			name: "overlap: only unique-to-a are returned",
			a:    []Vulnerability{vuln("CVE-1", "high"), vuln("CVE-2", "low")},
			b:    []Vulnerability{vuln("CVE-2", "low"), vuln("CVE-3", "medium")},
			want: []Vulnerability{vuln("CVE-1", "high")},
		},
		{
			name: "all overlap: empty result",
			a:    []Vulnerability{vuln("CVE-X", "critical")},
			b:    []Vulnerability{vuln("CVE-X", "critical")},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setDifference(tt.a, tt.b)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("setDifference() mismatch (-want +got):\n%s", d)
			}
		})
	}
}
