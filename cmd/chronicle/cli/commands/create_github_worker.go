package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/dependency/render"
	"github.com/anchore/chronicle/chronicle/dependency/scan"
	"github.com/anchore/chronicle/chronicle/dependency/source"
	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/releasers/github"
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

	ghConfig := appConfig.Github.ToGithubConfig()

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

	// optional source-scan dependency diff. Non-fatal: a changelog must not
	// fail because grype isn't ready or syft hit a snag.
	attachDependencyDiff(ctx, appConfig, gitter, untilTag, description, evidence.Leaf("source sbom"), evidence.Leaf("vulnerabilities"))

	return startRelease, description, nil
}

// attachDependencyDiff runs the opt-in dependency diff between the resolved
// since/until endpoints and attaches it to the description. Any failure (no DB,
// syft error, unresolvable ref) is logged and swallowed so changelog generation
// continues unaffected.
func attachDependencyDiff(ctx context.Context, appConfig *createConfig, gitter git.Interface, untilTag string, description *release.Description, sbomLeaf, vulnLeaf *event.Leaf) {
	if description == nil {
		return
	}

	// the feature is enabled when at least one ecosystem is requested.
	ecosystems := splitEcosystems(appConfig.Dependencies.Ecosystems)
	if len(ecosystems) == 0 {
		return
	}

	// since: prefer the previous release tag; otherwise the first commit so the
	// diff spans the whole history.
	sinceRef := ""
	if description.PreviousRelease != nil {
		sinceRef = description.PreviousRelease.Version
	}
	if sinceRef == "" {
		sha, err := gitter.FirstCommit()
		if err != nil {
			log.WithFields("error", err).Warn("unable to resolve since ref for dependency diff; skipping")
			return
		}
		sinceRef = sha
	}

	// until: the resolved end tag, else HEAD.
	untilRef := untilTag
	if untilRef == "" {
		untilRef = "HEAD"
	}

	if appConfig.Dependencies.OnlyVulnerable && !appConfig.Dependencies.AnnotateVulnerabilities {
		log.Warn("dependencies.only-vulnerable has no effect without annotate-vulnerabilities; showing all changes")
	}

	annotate := appConfig.Dependencies.AnnotateVulnerabilities

	// name the syft source after the project so artifact IDs are stable across the
	// tmpdir each ref is materialized into; fall back to the repo dir name.
	sourceName := bus.Repo() // "owner/repo"
	if sourceName == "" {
		sourceName = filepath.Base(appConfig.RepoPath)
	}

	// kick the UI elements ComputeDiff drives but no longer owns: catalog both
	// refs now, plus (when annotating) the parallel DB update. The observer fills
	// in per-ref progress; the parent leaves resolve from the diff post-return.
	sbomLeaf.SetStage("cataloging…")
	sbomLeaf.Child("since").Start()
	sbomLeaf.Child("until").Start()
	if annotate {
		vulnLeaf.SetStage("updating db…")
		vulnLeaf.Child("since").Start()
		vulnLeaf.Child("until").Start()
	}

	// always keep the grype DB fresh; not user-configurable.
	scanner := scan.NewScanner(ecosystems, appConfig.Dependencies.Exclude, annotate, true)
	diff, err := dependency.ComputeDiff(ctx, scanner, dependency.Config{
		Target:      source.NewGitTarget(appConfig.RepoPath),
		Comparer:    scan.NewVersionComparer(),
		Observer:    dependencyObserver(sbomLeaf, vulnLeaf),
		SinceRef:    sinceRef,
		UntilRef:    untilRef,
		SourceName:  sourceName,
		Annotate:    annotate,
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
	description.DependencyDiff = diff

	// resolve the parent leaves from the finished diff (the per-ref children were
	// resolved live by the observer). The vulnerability rollup is only meaningful
	// when both refs matched; if either failed, fail the parent rather than show a
	// hollow 0/0.
	sbomLeaf.Resolve(diffMetrics(diff)...)
	if annotate {
		if matchErr := firstLeafErr(vulnLeaf.Child("since"), vulnLeaf.Child("until")); matchErr != nil {
			vulnLeaf.Fail(matchErr)
		} else {
			vulnLeaf.Resolve(vulnMetrics(diff)...)
		}
	}

	// presentation travels alongside the data, not inside it. only-vulnerable is
	// a render-time filter and only meaningful once annotation has populated the
	// vuln deltas, so gate it on AnnotateVulnerabilities.
	rc := dependencyRenderConfig(appConfig.Dependencies)
	rc.OnlyVulnerable = appConfig.Dependencies.OnlyVulnerable && appConfig.Dependencies.AnnotateVulnerabilities
	description.DependencyRender = &rc
}

// dependencyObserver adapts ComputeDiff's per-ref progress callbacks onto the
// sbom/vuln evidence leaves. Each callback touches only its own ref's child leaf
// (matched by name to the "since"/"until" children), so the parallel scan
// goroutines never contend; the event leaves are themselves concurrency-safe.
func dependencyObserver(sbomLeaf, vulnLeaf *event.Leaf) dependency.Observer {
	return dependency.Observer{
		SourceResolved: func(ref, srcID string) {
			bus.RegisterSBOMScanSource(srcID, sbomLeaf.Child(ref))
		},
		RefCataloged: func(ref string, packages int) {
			sbomLeaf.Child(ref).Resolve(event.Count("package", packages))
		},
		RefCatalogFailed: func(ref string, err error) {
			sbomLeaf.Child(ref).Fail(err)
		},
		MatchStarted: func(ref string) {
			vulnLeaf.Child(ref).SetStage("matching…")
		},
		RefMatched: func(ref string, vulns int) {
			vulnLeaf.Child(ref).Resolve(event.Count("vulnerability", vulns))
		},
		RefMatchFailed: func(ref string, err error) {
			vulnLeaf.Child(ref).Fail(err)
		},
	}
}

// diffMetrics is the parent "source sbom" resolved figures: the package changes
// broken down by kind, rendered by the UI as a breakdown.
func diffMetrics(d *dependency.Diff) []event.Metric {
	t := d.Totals()
	return []event.Metric{
		event.Count("added", t.Added),
		event.Count("removed", t.Removed),
		event.Count("updated", t.Updated),
		event.Count("downgraded", t.Downgraded),
	}
}

// vulnMetrics is the parent "vulnerabilities" resolved figures: the net effect of
// the diff on vulnerabilities, rendered by the UI as a breakdown.
func vulnMetrics(d *dependency.Diff) []event.Metric {
	return []event.Metric{
		event.Count("remediated", d.RemediatedCount()),
		event.Count("introduced", d.IntroducedCount()),
	}
}

// firstLeafErr returns the first failed leaf's error, or nil when none failed.
func firstLeafErr(leaves ...*event.Leaf) error {
	for _, l := range leaves {
		if err := l.Err(); err != nil {
			return err
		}
	}
	return nil
}

// dependencyDiffEnabled reports whether the opt-in source-sbom dependency diff
// will run — i.e. at least one ecosystem was requested. Mirrors the guard in
// attachDependencyDiff so the evidence tree can reserve the "source sbom" leaf.
func dependencyDiffEnabled(appConfig *createConfig) bool {
	return len(splitEcosystems(appConfig.Dependencies.Ecosystems)) > 0
}

// splitEcosystems normalizes the --dependencies values: each entry may itself
// be comma-separated, so flatten, trim, and drop blanks.
func splitEcosystems(raw []string) []string {
	var out []string
	for _, entry := range raw {
		for _, part := range strings.Split(entry, ",") {
			if p := strings.TrimSpace(part); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
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
	if dependencyDiffEnabled(appConfig) {
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
