package commands

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/dependency/scan"
	"github.com/anchore/chronicle/chronicle/dependency/source"
	"github.com/anchore/chronicle/chronicle/dependency/toolchain"
	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/releasers/github"
	"github.com/anchore/chronicle/chronicle/release/render"
	"github.com/anchore/chronicle/cmd/chronicle/cli/options"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func createChangelogFromGithub(ctx context.Context, appConfig *createConfig) (*release.Release, *release.Description, error) {
	// pre-flight: the trunk format encodes commit-level data that is only
	// populated when consider-pr-merge-commits is enabled. Fail fast before
	// any GitHub API work if the combination is invalid.
	if err := checkTrunkPrerequisites(appConfig); err != nil {
		return nil, nil, err
	}

	ghConfig := buildGithubConfig(appConfig)

	gitter, err := git.New(appConfig.RepoPath)
	if err != nil {
		return nil, nil, err
	}

	summer, err := github.NewSummarizer(gitter, ghConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create summarizer: %w", err)
	}

	// surface the resolved repo identity to the bus so the post-teardown
	// summary can render the "chronicle vX · OWNER/REPO" header line that
	// matches what the live TUI showed.
	if owner, repo := summer.Repo(); owner != "" && repo != "" {
		bus.SetRepo(owner + "/" + repo)
	}

	// publish UI groups: range (since/until) and evidence (commits/issues/PRs).
	// The bus helpers return non-nil values whether or not a publisher is
	// attached, so calls below are safe regardless.
	rng := publishRangeGroup(appConfig)
	defer rng.Close()
	// flip slots to running immediately so the spinner is visible while the
	// since/until lookups and tag-discovery work runs. Resolve/Fail later will
	// transition them out.
	rng.Slot("since").Start()
	rng.Slot("until").Start()

	evidence := publishEvidenceTree(appConfig)
	defer evidence.Close()

	// when annotating, refresh the grype vulnerability DB now so a (possibly slow)
	// download overlaps the GitHub fetch below; a stale/missing DB spins the
	// "vulnerabilities" row on "updating DB" until matching takes over. The loaded
	// DB is awaited before the dependency scan. No-op when annotation is off.
	vulnLeaf := evidence.Leaf("vulnerabilities")
	dbRefresh := startVulnDBRefresh(appConfig, vulnLeaf)
	// safety net: if the dependency diff is skipped after the refresh started the
	// row spinning (e.g. refs can't resolve), leave the row terminal, not running.
	defer finalizeVulnLeaf(vulnLeaf)

	changeTypeTitles := getGithubSupportedChanges(appConfig)

	var untilTag = appConfig.UntilTag
	if untilTag == "" {
		untilTag, err = github.FindChangelogEndTag(summer, gitter)
		if err != nil {
			rng.Slot("until").Fail(err)
			return nil, nil, err
		}
	}

	if untilTag != "" {
		log.WithFields("tag", untilTag).Info("until the given tag")
	} else {
		log.Info("until the current revision (no end tag)")
	}

	changelogConfig := buildChangelogConfig(appConfig, untilTag, changeTypeTitles, evidence, gitter)

	startRelease, description, err := release.ChangelogInfo(summer, changelogConfig)
	if err != nil {
		return startRelease, description, err
	}

	// surface the configured conventional-commit prefixes so encoders can trim
	// them from change display text consistently with how they were categorized.
	if description != nil {
		description.ConventionalCommitTypes = getGithubConventionalCommitTypes(appConfig)
	}

	// resolve range slots from what we now know about each end of the range.
	resolveRangeSlots(rng, gitter, appConfig.SinceTag, untilTag, description)

	// surface raw fetch totals and resolve evidence leaves with kept counts.
	resolveEvidenceLeaves(evidence, summer, description)

	// enrich the description with the two opt-in diffs (toolchain + dependencies),
	// joined before returning so the description is fully populated.
	enrichDescription(ctx, appConfig, gitter, startRelease, untilTag, description, evidence, dbRefresh, vulnLeaf)

	return startRelease, description, nil
}

// buildGithubConfig derives the summarizer config from app config, suppressing
// dependency-bot PRs when the dependencies section is enabled since it reports
// those bumps directly (avoids double-reporting).
func buildGithubConfig(appConfig *createConfig) github.Config {
	ghConfig := appConfig.Github.ToGithubConfig()
	if appConfig.Dependencies.Enabled() {
		ghConfig.ExcludeAuthors = append(ghConfig.ExcludeAuthors, "dependabot", "renovate")
	}
	return ghConfig
}

// enrichDescription runs the two opt-in, description-enriching diffs concurrently.
// Toolchain detection only reads go.mod (etc.) at the two refs, so it is independent of
// the much heavier dependency scan and writes a separate field of the description; running
// it concurrently hides its latency behind the scan. Each gitter call opens its own repo
// handle, so the shared gitter is safe to use from both. Joined before returning.
func enrichDescription(ctx context.Context, appConfig *createConfig, gitter git.Interface, startRelease *release.Release, untilTag string, description *release.Description, evidence *event.Tree, dbRefresh <-chan *scan.DB, vulnLeaf *event.Leaf) {
	var wg sync.WaitGroup
	wg.Go(func() {
		// detect toolchain-requirement changes (opt-in) using the now-resolved range. The result
		// shares the evidence tree as one more row (nil leaf when detection is disabled) and is
		// rendered as a rollup within the Dependencies section.
		resolveToolchain(appConfig, gitter, startRelease, untilTag, description, evidence.Leaf("toolchain"))
	})

	// optional source-scan dependency diff. Non-fatal: a changelog must not
	// fail because grype isn't ready or syft hit a snag. Await the DB refresh
	// kicked off above (parallel with the fetch) and hand the loaded DB to the scan.
	db := awaitVulnDB(dbRefresh)
	attachDependencyDiff(ctx, appConfig, gitter, untilTag, description, db, evidence.Leaf("source sbom"), vulnLeaf)

	wg.Wait()
}

// attachDependencyDiff runs the opt-in dependency diff between the resolved
// since/until endpoints and attaches it to the description. Any failure (no DB,
// syft error, unresolvable ref) is logged and swallowed so changelog generation
// continues unaffected.
func attachDependencyDiff(ctx context.Context, appConfig *createConfig, gitter git.Interface, untilTag string, description *release.Description, db *scan.DB, sbomLeaf, vulnLeaf *event.Leaf) {
	if description == nil {
		return
	}

	// the feature is enabled when at least one ecosystem is requested.
	ecosystems := appConfig.Dependencies.CleanedEcosystems()
	if len(ecosystems) == 0 {
		return
	}

	sinceRef, untilRef, ok := resolveDependencyRefs(description, gitter, untilTag)
	if !ok {
		return
	}

	configured := appConfig.Dependencies.AnnotateVulnerabilities
	annotate := configured
	// gracefully degrade when annotation was requested but no usable DB loaded
	// (missing/corrupt, or a failed download): behave exactly as if
	// annotate-vulnerabilities was never passed — packages-only, with the row
	// skipped rather than failed. The cause was already warned at load time.
	if configured && db == nil {
		annotate = false
		skipVulnLeaf(vulnLeaf)
	}

	// only-vulnerable is a render-time filter that only means anything once
	// annotation has populated the vuln deltas. Resolve it against the effective
	// annotate state; warn only when the user asked for it without configuring
	// annotation at all (a DB-unavailable degrade is already explained above).
	onlyVulnerable := appConfig.Dependencies.OnlyVulnerable && annotate
	if appConfig.Dependencies.OnlyVulnerable && !configured {
		log.Warn("dependencies.only-vulnerable has no effect without annotate-vulnerabilities; showing all changes")
	}

	// remaining (carried-over) vulnerabilities likewise only exist once annotation
	// has matched both refs; gate it the same way and warn on a no-op request.
	showRemaining := appConfig.Dependencies.ShowRemainingVulnerabilities && annotate
	if appConfig.Dependencies.ShowRemainingVulnerabilities && !configured {
		log.Warn("dependencies.show-remaining-vulnerabilities has no effect without annotate-vulnerabilities; omitting remaining vulnerabilities")
	}

	// name the syft source after the project so artifact IDs are stable across the
	// tmpdir each ref is materialized into; fall back to the repo dir name.
	sourceName := bus.Repo() // "owner/repo"
	if sourceName == "" {
		sourceName = filepath.Base(appConfig.RepoPath)
	}

	startDependencyLeaves(sbomLeaf, vulnLeaf, sinceRef, untilRef, annotate)

	// the scanner owns materialization (the git Target) and matches against the
	// pre-loaded DB (refreshed in parallel with the GitHub fetch); a nil db scans
	// packages only, and ComputeDiff infers whether to attribute from the data.
	scanner := scan.NewScanner(source.NewGitTarget(appConfig.RepoPath), sourceName, ecosystems, appConfig.Dependencies.Exclude, appConfig.Dependencies.Recursive, db)
	result, err := dependency.ComputeDiff(ctx, scanner, dependency.DiffConfig{
		Comparer:    scan.NewVersionComparer(),
		SinceRef:    sinceRef,
		UntilRef:    untilRef,
		MinSeverity: appConfig.Dependencies.MinSeverity,
	})
	if err != nil {
		log.WithFields("error", err).Warn("unable to compute dependency diff; continuing without it")
		sbomLeaf.Fail(err)
		if annotate {
			vulnLeaf.Fail(err)
		}
		return
	}
	description.DependencyDiff = result
	resolveDependencyLeaves(sbomLeaf, vulnLeaf, result, annotate)

	// presentation travels alongside the data, not inside it.
	rc := dependencyRenderConfig(appConfig.Dependencies)
	rc.OnlyVulnerable = onlyVulnerable
	rc.ShowRemaining = showRemaining
	description.DependencyRender = &rc
}

// vulnDBMaxAge is how stale the grype vulnerability DB may be before chronicle
// acts on it: when DB updates are enabled, a DB older than this (or missing)
// triggers a download; when updates are disabled, an older DB is used as-is with
// a warning. Matches grype's own max-allowed DB age.
const vulnDBMaxAge = 5 * 24 * time.Hour

// startVulnDBRefresh loads the grype vulnerability DB in the background so a
// (possibly slow) download overlaps the commit/issue/PR fetch. It returns nil
// when vulnerability annotation is off. With DB updates enabled (the default), a
// missing or stale DB spins the "vulnerabilities" row on "updating DB" while it
// downloads; with updates disabled, the on-disk DB is loaded as-is and a stale
// one only logs a warning (a missing/unusable DB then degrades to packages-only).
// awaitVulnDB joins the loaded DB before the dependency scan matches — the row's
// matching/resolve states are driven later, as usual.
func startVulnDBRefresh(appConfig *createConfig, vulnLeaf *event.Leaf) <-chan *scan.DB {
	if !appConfig.Dependencies.AnnotateVulnerabilities || !appConfig.Dependencies.Enabled() {
		return nil
	}

	// a quick, local status read (no network) decides whether to update and/or
	// warn. We only download when the DB is stale/missing AND updates are enabled;
	// otherwise a stale-but-present DB is used as-is with a warning.
	present, age := scan.DBStatus()
	stale := !present || age > vulnDBMaxAge
	update := stale && appConfig.Dependencies.UpdateVulnerabilityDB
	switch {
	case update:
		// light up the row only when grype actually downloads; SetStage flips the
		// pending row to running.
		vulnLeaf.SetStage("updating DB")
	case present && age > vulnDBMaxAge:
		log.WithFields("age", age.Round(time.Hour).String()).
			Warn("vulnerability DB is older than the max recommended age and DB updates are disabled; results may be stale")
	}

	ch := make(chan *scan.DB, 1)
	go func() {
		db, err := scan.LoadDB(update)
		if err != nil {
			// non-fatal: the scan degrades to packages-only and the vulnerability
			// row is skipped downstream (db is nil → attachDependencyDiff degrades).
			log.WithFields("error", err).Warn("unable to load vulnerability DB; continuing without vulnerability annotations")
		}
		ch <- db
	}()
	return ch
}

// awaitVulnDB blocks for the background DB refresh and returns the loaded DB. It
// returns nil when annotation is off or the refresh failed — the scan then
// degrades to packages-only and the vulnerability leaves are failed downstream.
func awaitVulnDB(ch <-chan *scan.DB) *scan.DB {
	if ch == nil {
		return nil
	}
	return <-ch
}

// skipVulnLeaf marks the "vulnerabilities" row and its since/until branches as
// skipped so they read as intentionally-not-done (⊘) rather than being promoted
// to a hollow resolved checkmark when the tree closes. Used when annotation was
// requested but no usable DB loaded, so the run degrades to packages-only.
func skipVulnLeaf(vulnLeaf *event.Leaf) {
	for _, child := range vulnLeaf.Children() {
		child.Skip()
	}
	vulnLeaf.Skip()
}

// finalizeVulnLeaf is a safety net for the "vulnerabilities" row: the DB refresh
// may have started it spinning ("updating DB") before the dependency diff ran. If
// the diff was skipped (so nothing resolved or failed the row), skip it here so it
// doesn't linger in a running state. A no-op once the row has reached a terminal
// state in the normal flow.
func finalizeVulnLeaf(vulnLeaf *event.Leaf) {
	if vulnLeaf != nil && vulnLeaf.State() == event.SlotRunning {
		vulnLeaf.Skip()
	}
}

// resolveDependencyRefs picks the since/until endpoints for the dependency diff:
// since is the previous release tag, or the first commit when there is none (so
// the diff spans all history); until is the resolved end tag, or HEAD. ok is
// false when the since ref can't be resolved, signaling the caller to skip.
func resolveDependencyRefs(description *release.Description, gitter git.Interface, untilTag string) (sinceRef, untilRef string, ok bool) {
	if description.PreviousRelease != nil {
		sinceRef = description.PreviousRelease.Version
	}
	if sinceRef == "" {
		sha, err := gitter.FirstCommit()
		if err != nil {
			log.WithFields("error", err).Warn("unable to resolve since ref for dependency diff; skipping")
			return "", "", false
		}
		sinceRef = sha
	}

	untilRef = untilTag
	if untilRef == "" {
		untilRef = "HEAD" //nolint:goconst // git ref literal; clearer inline than named
	}
	return sinceRef, untilRef, true
}

// startDependencyLeaves registers each ref's sbom branch leaf so the scan can
// route syft's live package count onto the right branch (it publishes its
// resolved source to the bus from deep inside), then kicks the spinners.
func startDependencyLeaves(sbomLeaf, vulnLeaf *event.Leaf, sinceRef, untilRef string, annotate bool) {
	bus.RegisterSBOMLeaf(sinceRef, sbomLeaf.Child("since"))
	bus.RegisterSBOMLeaf(untilRef, sbomLeaf.Child("until"))
	sbomLeaf.SetStage("cataloging…")
	sbomLeaf.Child("since").Start()
	sbomLeaf.Child("until").Start()
	if annotate {
		vulnLeaf.SetStage("matching…")
		vulnLeaf.Child("since").Start()
		vulnLeaf.Child("until").Start()
	}
}

// resolveDependencyLeaves renders the scan's authoritative figures onto the
// leaves: each ref's package (and vulnerability) count, plus the diff rollups on
// the parents. The scan published only its live progress; these final counts
// come back as data, mirroring how resolveEvidenceLeaves resolves from the
// returned description.
func resolveDependencyLeaves(sbomLeaf, vulnLeaf *event.Leaf, diff *dependency.Diff, annotate bool) {
	sbomLeaf.Child("since").Resolve(event.Count("package", diff.Since.DistinctPackages()))
	sbomLeaf.Child("until").Resolve(event.Count("package", diff.Until.DistinctPackages()))
	sbomLeaf.Resolve(diffMetrics(diff)...)
	if !annotate {
		return
	}
	resolveVulnBranch(vulnLeaf.Child("since"), diff.Since)
	resolveVulnBranch(vulnLeaf.Child("until"), diff.Until)
	// the rollup is only meaningful when both refs matched; a nil Vulns map means
	// matching didn't complete for that ref, so fail the parent rather than show
	// a hollow 0/0.
	if diff.Since.Vulns == nil || diff.Until.Vulns == nil {
		vulnLeaf.Fail(errVulnMatchIncomplete)
	} else {
		vulnLeaf.Resolve(vulnMetrics(diff)...)
	}
}

// errVulnMatchIncomplete fails a vulnerability leaf when matching didn't complete
// for a ref. The specific cause (DB unavailable, matcher error) is logged by the
// scanner; the leaf just needs a terminal failed state.
var errVulnMatchIncomplete = errors.New("vulnerability matching did not complete")

// resolveVulnBranch resolves a vulnerability branch leaf from its ref scan:
// the distinct match count, or a failure when matching didn't complete (nil
// Vulns map).
func resolveVulnBranch(leaf *event.Leaf, snap dependency.Scan) {
	if snap.Vulns == nil {
		leaf.Fail(errVulnMatchIncomplete)
		return
	}
	leaf.Resolve(event.Count("vulnerability", snap.DistinctVulns()))
}

// diffMetrics is the parent "source sbom" resolved figures: the package changes
// broken down by kind, rendered by the UI as a breakdown.
func diffMetrics(d *dependency.Diff) []event.Metric {
	t := d.Totals
	return []event.Metric{
		event.Count("added", t.Added),
		event.Count("removed", t.Removed),
		event.Count("updated", t.Updated),
		event.Count("downgraded", t.Downgraded),
	}
}

// vulnMetrics is the parent "vulnerabilities" resolved figures: the net effect of
// the diff on vulnerabilities, rendered by the UI as a breakdown. Remaining is
// the carried-over burden the release did not clear, included alongside the
// delta so the breakdown tells the whole story (it is always computed when both
// refs match, independent of whether show-remaining renders it).
func vulnMetrics(d *dependency.Diff) []event.Metric {
	return []event.Metric{
		event.Count("remediated", d.RemediatedCount),
		event.Count("introduced", d.IntroducedCount),
		event.Count("remaining", d.RemainingCount),
	}
}

// dependencyRenderConfig maps the cmd-layer dependency options onto the core
// render config consumed by the encoders. Each action is a comma-separated
// fallback list (e.g. "collapsed,list"); an empty/invalid value leaves nil so
// ModesFor resolves the per-kind default.
func dependencyRenderConfig(d options.Dependencies) render.Config {
	return render.Config{
		Actions: map[dependency.ChangeKind][]render.Mode{
			dependency.Updated:    render.ParseModes(d.Actions.Updated),
			dependency.Downgraded: render.ParseModes(d.Actions.Downgraded),
			dependency.Added:      render.ParseModes(d.Actions.Added),
			dependency.Removed:    render.ParseModes(d.Actions.Removed),
		},
	}
}

// toolchainConfig builds the toolchain detection config from the dependencies options. Detection
// rides on the dependencies feature: it runs only when --dependencies is active and the
// detect-toolchain toggle is on, and only covers the activated ecosystems that have a detector.
// The standard ignore globs are extended with the dependencies `exclude` list so both features
// skip the same vendored/test trees.
func toolchainConfig(appConfig *createConfig) toolchain.Config {
	if !appConfig.Dependencies.Enabled() || !appConfig.Dependencies.DetectToolchain {
		return toolchain.Config{}
	}
	ecos := toolchainEcosystems(appConfig.Dependencies.CleanedEcosystems())
	if len(ecos) == 0 {
		return toolchain.Config{}
	}
	return toolchain.Config{
		Enabled:    true,
		Ecosystems: ecos,
		Ignore:     append(toolchain.DefaultIgnore(), appConfig.Dependencies.Exclude...),
		Recursive:  appConfig.Dependencies.Recursive,
	}
}

// toolchainEcosystems maps the activated dependency ecosystems (syft cataloger selectors) to the
// toolchain ecosystems we have detectors for. The "language" meta-selector expands to every known
// toolchain ecosystem; a selector that parses to an ecosystem with no detector (e.g. "java") or
// does not parse at all is dropped. The result is deduplicated and ordered by KnownEcosystems.
func toolchainEcosystems(depEcosystems []string) []dependency.Ecosystem {
	known := make(map[dependency.Ecosystem]bool)
	for _, e := range toolchain.KnownEcosystems() {
		known[e] = true
	}

	want := make(map[dependency.Ecosystem]bool)
	for _, sel := range depEcosystems {
		if strings.EqualFold(strings.TrimSpace(sel), "language") {
			for e := range known {
				want[e] = true
			}
			continue
		}
		if e, ok := dependency.ParseEcosystem(sel); ok && known[e] {
			want[e] = true
		}
	}

	var out []dependency.Ecosystem
	for _, e := range toolchain.KnownEcosystems() {
		if want[e] {
			out = append(out, e)
		}
	}
	return out
}

// resolveToolchain runs toolchain detection (when enabled), drives its row in the evidence tree,
// and attaches the result to the description. Detection is best-effort: any failure is logged and
// does not abort changelog generation. Reconciliation/downgrade warnings are surfaced to the
// operator log here. The leaf is nil (a no-op) when detection is disabled.
func resolveToolchain(appConfig *createConfig, gitter git.Interface, startRelease *release.Release, untilTag string, description *release.Description, leaf *event.Leaf) {
	cfg := toolchainConfig(appConfig)
	if !cfg.Enabled || description == nil {
		return
	}

	leaf.SetStage("inspecting sources")

	sinceRef := appConfig.SinceTag
	if sinceRef == "" && startRelease != nil {
		sinceRef = startRelease.Version
	}

	untilRef := untilTag
	if untilRef == "" {
		untilRef = "HEAD"
		// detection diffs committed objects, so working-tree edits to a manifest are invisible.
		// when ending at HEAD, warn if a toolchain source file is dirty so a bump that only exists
		// uncommitted isn't mistaken for "no change".
		warnOnUncommittedToolchainChanges(gitter, cfg)
	}

	if sinceRef == "" {
		// no baseline to diff against (changelog starts at the beginning of git history).
		leaf.Skip()
		return
	}

	data, err := toolchain.Detect(gitter, cfg, sinceRef, untilRef)
	if err != nil {
		leaf.Fail(err)
		log.WithFields("error", err).Warn("toolchain detection failed")
		return
	}

	if data == nil {
		// detection succeeded but found no toolchain changes between the two refs.
		leaf.Resolve(event.Count("change", 0))
		return
	}

	description.Toolchain = data
	// match the other evidence rows: a single named count the UI pluralizes
	// ("1 change", "2 changes"). Downgrade/conflict detail goes to the log below.
	leaf.Resolve(event.Count("change", len(data.Updates)))
	logToolchainWarnings(data)
}

// logToolchainWarnings surfaces reconciliation conflicts and downgrades to the operator log,
// beyond the inline annotations in the rendered changelog.
func logToolchainWarnings(data *release.ToolchainData) {
	if data == nil {
		return
	}
	for _, w := range data.Warnings {
		log.WithFields("tool", w.Tool, "files", strings.Join(w.Files, ", ")).Warn(w.Message)
	}
	for _, u := range data.Updates {
		if u.Direction == release.ToolchainDowngrade {
			log.WithFields("tool", u.Tool, "file", u.File, "from", u.From, "to", u.To).
				Warn("toolchain minimum version was downgraded")
		}
	}
}

// warnOnUncommittedToolchainChanges warns when the changelog ends at HEAD but toolchain source
// files have uncommitted working-tree changes (which the committed-history diff cannot see). The
// check is best-effort — any failure is logged at trace level and otherwise ignored.
func warnOnUncommittedToolchainChanges(gitter git.Interface, cfg toolchain.Config) {
	dirty, err := toolchain.DirtySourceFiles(gitter, cfg)
	if err != nil {
		log.WithFields("error", err).Trace("unable to check working tree for uncommitted toolchain changes")
		return
	}
	if len(dirty) == 0 {
		return
	}
	log.WithFields("files", strings.Join(dirty, ", ")).
		Warn("toolchain detection ends at HEAD but these source files have uncommitted changes; any toolchain version change in them will not appear in the changelog until committed")
}

// buildChangelogConfig assembles the ChangelogInfoConfig, including an
// optional speculator when --speculate-next-version was set.
func buildChangelogConfig(appConfig *createConfig, untilTag string, titles []change.TypeTitle, evidence *event.Tree, gitter git.Interface) release.ChangelogInfoConfig {
	var speculator release.VersionSpeculator
	if appConfig.SpeculateNextVersion {
		speculator = github.NewVersionSpeculator(gitter, release.SpeculationBehavior{
			EnforceV0:           bool(appConfig.EnforceV0),
			NoChangesBumpsPatch: true,
		})
	}
	return release.ChangelogInfoConfig{
		RepoPath:          appConfig.RepoPath,
		SinceTag:          appConfig.SinceTag,
		UntilTag:          untilTag,
		VersionSpeculator: speculator,
		ChangeTypeTitles:  titles,
		CommitsLeaf:       evidence.Leaf("commits"),
		IssuesLeaf:        evidence.Leaf("issues"),
		PRsLeaf:           evidence.Leaf("pull requests"),
	}
}

// publishEvidenceTree builds and publishes the "evidence" tree (commits, issues,
// PRs) and kicks each base leaf into the running state so spinners show during
// the GraphQL fetches. When the dependency diff is enabled it adds a "source
// sbom" leaf with since/until branches (driven later via the ComputeDiff
// observer), plus a sibling "vulnerabilities" leaf when annotation is on. The
// caller owns Close.
func publishEvidenceTree(appConfig *createConfig) *event.Tree {
	evidenceSpecs := []event.LeafSpec{
		{Name: "commits"},
		{Name: "issues"},
		{Name: "pull requests"},
	}
	if appConfig.Dependencies.Enabled() {
		evidenceSpecs = append(evidenceSpecs, event.LeafSpec{
			Name:     "source sbom",
			Children: []string{"since", "until"},
		})
		if appConfig.Dependencies.AnnotateVulnerabilities {
			evidenceSpecs = append(evidenceSpecs, event.LeafSpec{
				Name:     "vulnerabilities",
				Children: []string{"since", "until"},
			})
		}
		if toolchainConfig(appConfig).Enabled {
			// toolchain detection rides on the dependencies feature, so it shares the evidence
			// tree as one more row. It stays pending until detection runs at the end of the flow.
			evidenceSpecs = append(evidenceSpecs, event.LeafSpec{Name: "toolchain"})
		}
	}
	evidence := bus.PublishTreeSpec("evidence", evidenceSpecs)
	evidence.Leaf("commits").Start()
	evidence.Leaf("issues").Start()
	evidence.Leaf("pull requests").Start()
	return evidence
}

// resolveEvidenceLeaves copies fetch totals onto the description and resolves
// the three evidence leaves. The trailer reports how many of the fetched items
// were *dropped* — i.e., not associated with the release directly or
// indirectly. A row with nothing dropped renders without any trailer to keep
// the eye on the count itself.
func resolveEvidenceLeaves(evidence *event.Tree, summer *github.Summarizer, description *release.Description) {
	prTotal, issueTotal, commitTotal := summer.EvidenceTotals()
	if description != nil {
		description.PRTotal = prTotal
		description.IssueTotal = issueTotal
		description.CommitTotal = commitTotal
	}

	// commits is always resolved with its count: it is the signal we acted on
	// (zero commits is what drove the short-circuit). The dropped trailer (how
	// many fetched items aren't associated with the release) is raw — the UI
	// decides whether and how to show it.
	commits := evidence.Leaf("commits")
	commits.Resolve(event.Num(commitTotal))
	commits.SetDropped(commitTotal - summer.AssociatedCommits())

	// when there were no commits in scope the issue/PR fetches were skipped, so
	// mark those leaves as skipped rather than resolved-with-zero — a zero count
	// would imply we looked and found nothing.
	if summer.DetailFetchSkipped() {
		evidence.Leaf("issues").Skip()
		evidence.Leaf("pull requests").Skip()
		return
	}

	issues := evidence.Leaf("issues")
	issues.Resolve(event.Num(issueTotal))
	issues.SetDropped(issueTotal - summer.IssuesKept())

	prs := evidence.Leaf("pull requests")
	prs.Resolve(event.Num(prTotal))
	prs.SetDropped(prTotal - summer.PRsKept())
}

// publishRangeGroup constructs the user-visible "range" bracket group with the
// since/until intent strings derived from user input.
func publishRangeGroup(appConfig *createConfig) *event.Group {
	sinceIntent := appConfig.SinceTag
	if sinceIntent == "" {
		sinceIntent = "latest release"
	}
	untilIntent := appConfig.UntilTag
	if untilIntent == "" {
		untilIntent = "HEAD"
	}
	return bus.PublishGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: sinceIntent},
		{Name: "until", Label: "until", Intent: untilIntent},
	})
}

// resolveRangeSlots populates the since/until slot values from the resolved
// release/tag information now available post-ChangelogInfo. Any lookup error
// here is non-fatal — we still want a partially-populated slot rather than a
// failed one when at least the date or sha is known.
func resolveRangeSlots(rng *event.Group, gitter git.Interface, sinceTag, untilTag string, desc *release.Description) {
	// since: prefer a tag lookup; fall back to whatever PreviousRelease carries.
	// Values are raw (tag, full sha, timestamp); the UI shortens the sha and
	// formats the date.
	switch {
	case sinceTag != "":
		if t, err := gitter.SearchForTag(sinceTag); err == nil && t != nil {
			rng.Slot("since").Resolve(event.Text(t.Name), event.SHA(t.Commit), event.Date(t.Timestamp))
		} else {
			rng.Slot("since").Resolve(event.Text(sinceTag))
		}
	case desc != nil && desc.PreviousRelease != nil:
		ver := desc.PreviousRelease.Version
		if t, err := gitter.SearchForTag(ver); err == nil && t != nil {
			rng.Slot("since").Resolve(event.Text(ver), event.SHA(t.Commit), event.Date(desc.PreviousRelease.Date))
		} else {
			rng.Slot("since").Resolve(event.Text(ver), event.Date(desc.PreviousRelease.Date))
		}
	default:
		// no prior release: since the beginning of git history.
		if sha, err := gitter.FirstCommit(); err == nil {
			rng.Slot("since").Resolve(event.SHA(sha))
		}
	}

	// until: prefer the resolved tag; otherwise show HEAD.
	if untilTag != "" {
		if t, err := gitter.SearchForTag(untilTag); err == nil && t != nil {
			rng.Slot("until").Resolve(event.Text(t.Name), event.SHA(t.Commit), event.Date(t.Timestamp))
		} else {
			rng.Slot("until").Resolve(event.Text(untilTag))
		}
	} else if sha, err := gitter.HeadTagOrCommit(); err == nil {
		rng.Slot("until").Resolve(event.SHA(sha))
	}
}

// checkTrunkPrerequisites returns an error when the trunk output format is
// selected but consider-pr-merge-commits is disabled. The trunk encoder
// requires commit-level data that is only collected when that setting is on.
func checkTrunkPrerequisites(appConfig *createConfig) error {
	specs, err := appConfig.Specs()
	if err != nil {
		return err
	}

	for _, spec := range specs {
		if spec.Name == "trunk" && !appConfig.Github.ConsiderPRMergeCommits {
			return fmt.Errorf(`the "trunk" output format requires "consider-pr-merge-commits=true"; either enable it (or pass --consider-pr-merge-commits) or remove "-o trunk"`)
		}
	}
	return nil
}

// getGithubConventionalCommitTypes collects every conventional-commit type
// prefix configured across all change types (excluding the breaking "!" marker,
// which is not a type token). Encoders use these to trim non-standard prefixes
// from display text.
func getGithubConventionalCommitTypes(appConfig *createConfig) []string {
	var prefixes []string
	for _, c := range appConfig.Github.Changes {
		for _, p := range c.Prefixes {
			if p == change.BreakingChangePrefix {
				continue
			}
			prefixes = append(prefixes, p)
		}
	}
	return prefixes
}

func getGithubSupportedChanges(appConfig *createConfig) []change.TypeTitle {
	var supportedChanges []change.TypeTitle
	for _, c := range appConfig.Github.Changes {
		// TODO: this could be one source of truth upstream
		k := change.ParseSemVerKind(c.SemVerKind)
		t := change.NewType(c.Type, k)
		supportedChanges = append(supportedChanges, change.TypeTitle{
			ChangeType: t,
			Title:      c.Title,
		})
	}
	return supportedChanges
}
