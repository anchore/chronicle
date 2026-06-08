package dependency

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
)

// intComparer parses versions as plain integers: larger integer = newer. It
// returns ok=false for any string that cannot be parsed as an integer, which
// lets tests exercise the "unknown direction → Updated" path.
type intComparer struct{}

func (intComparer) Compare(_ string, a, b string) (int, bool) {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr != nil || berr != nil {
		return 0, false
	}
	// honor the VersionComparer contract: <0 when a<b.
	return ai - bi, true
}

// alwaysUnknownComparer always returns ok=false, simulating an ecosystem where
// Compare cannot determine version ordering.
type alwaysUnknownComparer struct{}

func (alwaysUnknownComparer) Compare(_ string, _, _ string) (int, bool) {
	return 0, false
}

func pkg(name, version, pkgType string) Package {
	return Package{Name: name, Version: version, Type: pkgType}
}

func snapshot(pkgs ...Package) Snapshot {
	return Snapshot{Packages: pkgs}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name  string
		since Snapshot
		until Snapshot
		cmp   VersionComparer
		want  []PackageChange
	}{
		{
			name:  "empty snapshots produce empty diff",
			since: snapshot(),
			until: snapshot(),
			cmp:   intComparer{},
			want:  nil,
		},
		{
			name:  "package only in since is removed",
			since: snapshot(pkg("foo", "1", "go-module")),
			until: snapshot(),
			cmp:   intComparer{},
			want: []PackageChange{
				{Name: "foo", Type: "go-module", FromVersion: "1", ToVersion: "", Kind: Removed},
			},
		},
		{
			name:  "package only in until is added",
			since: snapshot(),
			until: snapshot(pkg("bar", "2", "go-module")),
			cmp:   intComparer{},
			want: []PackageChange{
				{Name: "bar", Type: "go-module", FromVersion: "", ToVersion: "2", Kind: Added},
			},
		},
		{
			name:  "version increase is updated",
			since: snapshot(pkg("lib", "1", "npm")),
			until: snapshot(pkg("lib", "2", "npm")),
			cmp:   intComparer{},
			want: []PackageChange{
				{Name: "lib", Type: "npm", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
		},
		{
			name:  "version decrease is downgraded",
			since: snapshot(pkg("lib", "3", "npm")),
			until: snapshot(pkg("lib", "1", "npm")),
			cmp:   intComparer{},
			want: []PackageChange{
				{Name: "lib", Type: "npm", FromVersion: "3", ToVersion: "1", Kind: Downgraded},
			},
		},
		{
			name:  "equal version is omitted",
			since: snapshot(pkg("stable", "5", "go-module")),
			until: snapshot(pkg("stable", "5", "go-module")),
			cmp:   intComparer{},
			want:  nil,
		},
		{
			name:  "unknown direction (ok=false) classifies as updated",
			since: snapshot(pkg("lib", "v1.0.0", "go-module")),
			until: snapshot(pkg("lib", "v2.0.0", "go-module")),
			cmp:   alwaysUnknownComparer{},
			want: []PackageChange{
				{Name: "lib", Type: "go-module", FromVersion: "v1.0.0", ToVersion: "v2.0.0", Kind: Updated},
			},
		},
		{
			name: "multiple packages sorted deterministically by type then name",
			since: snapshot(
				pkg("z-pkg", "1", "npm"),
				pkg("a-pkg", "1", "go-module"),
				pkg("m-pkg", "1", "npm"),
			),
			until: snapshot(
				pkg("z-pkg", "2", "npm"),
				pkg("a-pkg", "2", "go-module"),
				pkg("m-pkg", "2", "npm"),
			),
			cmp: intComparer{},
			want: []PackageChange{
				{Name: "a-pkg", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
				{Name: "m-pkg", Type: "npm", FromVersion: "1", ToVersion: "2", Kind: Updated},
				{Name: "z-pkg", Type: "npm", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
		},
		{
			name: "mixed changes: add, remove, update, downgrade, unchanged",
			since: snapshot(
				pkg("removed-pkg", "1", "go-module"),
				pkg("updated-pkg", "1", "go-module"),
				pkg("downgraded-pkg", "5", "go-module"),
				pkg("unchanged-pkg", "3", "go-module"),
			),
			until: snapshot(
				pkg("added-pkg", "1", "go-module"),
				pkg("updated-pkg", "2", "go-module"),
				pkg("downgraded-pkg", "2", "go-module"),
				pkg("unchanged-pkg", "3", "go-module"),
			),
			cmp: intComparer{},
			want: []PackageChange{
				{Name: "added-pkg", Type: "go-module", FromVersion: "", ToVersion: "1", Kind: Added},
				{Name: "downgraded-pkg", Type: "go-module", FromVersion: "5", ToVersion: "2", Kind: Downgraded},
				{Name: "removed-pkg", Type: "go-module", FromVersion: "1", ToVersion: "", Kind: Removed},
				{Name: "updated-pkg", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
		},
		{
			name: "packages with same name but different types are distinct",
			since: snapshot(
				pkg("shared-name", "1", "npm"),
				pkg("shared-name", "1", "go-module"),
			),
			until: snapshot(
				pkg("shared-name", "2", "npm"),
				pkg("shared-name", "2", "go-module"),
			),
			cmp: intComparer{},
			want: []PackageChange{
				{Name: "shared-name", Type: "go-module", FromVersion: "1", ToVersion: "2", Kind: Updated},
				{Name: "shared-name", Type: "npm", FromVersion: "1", ToVersion: "2", Kind: Updated},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compare(tt.since, tt.until, tt.cmp)

			// Compare establishes the structural diff only — no annotation, so
			// every change's Vuln is nil. EquateEmpty treats the empty diff's
			// changes (non-nil empty) as equal to the nil want.
			if d := cmp.Diff(tt.want, got.Changes(), cmpopts.EquateEmpty()); d != "" {
				t.Errorf("Compare() changes mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestClassifyVersionChange(t *testing.T) {
	tests := []struct {
		name    string
		pkgType string
		from    string
		to      string
		cmp     VersionComparer
		want    ChangeKind
	}{
		{
			name: "updated when to > from",
			from: "1", to: "5",
			cmp:  intComparer{},
			want: Updated,
		},
		{
			name: "downgraded when to < from",
			from: "5", to: "1",
			cmp:  intComparer{},
			want: Downgraded,
		},
		{
			name: "unknown direction defaults to updated",
			from: "v1.0", to: "v2.0",
			cmp:  alwaysUnknownComparer{},
			want: Updated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyVersionChange(tt.pkgType, tt.from, tt.to, tt.cmp)
			require.Equal(t, tt.want, got)
		})
	}
}
