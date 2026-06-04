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
		since          Snapshot
		until          Snapshot
		cfg            AnnotateConfig
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
			since: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-001", "high"), vuln("CVE-002", "medium")}}),
			},
			until: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-002", "medium"), vuln("CVE-003", "low")}}),
			},
			cfg: AnnotateConfig{},
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
			since: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("old"), []Vulnerability{vuln("CVE-A", "critical"), vuln("CVE-B", "low")}}),
			},
			until: Snapshot{Vulns: nil},
			cfg:   AnnotateConfig{},
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
			since: Snapshot{Vulns: nil},
			until: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("new"), []Vulnerability{vuln("GHSA-XYZ", "high")}}),
			},
			cfg: AnnotateConfig{},
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
			since: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-OLD", "high")}}),
			},
			until: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{vuln("CVE-OLD", "high"), vuln("CVE-NEW", "critical")}}),
			},
			cfg: AnnotateConfig{},
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
			since: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{
					vuln("CVE-HIGH", "high"),
					vuln("CVE-LOW", "low"),
					vuln("CVE-NEG", "negligible"),
				}}),
			},
			until: Snapshot{Vulns: nil},
			cfg:   AnnotateConfig{MinSeverity: "high"},
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
			since: Snapshot{
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
			until: Snapshot{Vulns: nil},
			cfg:   AnnotateConfig{},
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
			since: Snapshot{Vulns: nil},
			until: Snapshot{Vulns: nil},
			cfg:   AnnotateConfig{},
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
			since: Snapshot{
				Vulns: vulnMap(struct {
					key   PackageKey
					vulns []Vulnerability
				}{goKey("lib"), []Vulnerability{
					vuln("CVE-NEG", "negligible"),
					vuln("CVE-UNK", ""),
				}}),
			},
			until: Snapshot{Vulns: nil},
			cfg:   AnnotateConfig{MinSeverity: ""},
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
			got := Annotate(NewDiff(tt.changes), tt.since, tt.until, tt.cfg)

			if d := cmp.Diff(tt.wantChanges, got.Changes()); d != "" {
				t.Errorf("Annotate() changes mismatch (-want +got):\n%s", d)
			}
			require.Equal(t, tt.wantRemediated, got.RemediatedCount())
			require.Equal(t, tt.wantIntroduced, got.IntroducedCount())
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
