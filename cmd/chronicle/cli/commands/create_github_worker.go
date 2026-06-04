package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/releasers/github"
	"github.com/anchore/chronicle/chronicle/release/toolchain"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func createChangelogFromGithub(appConfig *createConfig) (*release.Release, *release.Description, error) {
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

	evidenceLeaves := []string{"commits", "issues", "pull requests"}
	if appConfig.Toolchain.Enabled {
		// toolchain detection is another signal gathered for the changelog, so it shares the
		// evidence tree as one more row (only when enabled).
		evidenceLeaves = append(evidenceLeaves, "toolchain")
	}
	evidence := bus.PublishTree("evidence", evidenceLeaves)
	defer evidence.Close()
	// same kickoff for evidence — leaves stay running through the GraphQL
	// fetches until resolveEvidenceLeaves resolves them with their counts. The
	// toolchain leaf (if any) stays pending until detection runs at the end.
	evidence.Leaf("commits").Start()
	evidence.Leaf("issues").Start()
	evidence.Leaf("pull requests").Start()

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

	// resolve range slots from what we now know about each end of the range.
	resolveRangeSlots(rng, gitter, appConfig.SinceTag, untilTag, description)

	// surface raw fetch totals and resolve evidence leaves with kept counts.
	resolveEvidenceLeaves(evidence, summer, description)

	// detect toolchain-requirement changes (opt-in) using the now-resolved range. The result
	// shares the evidence tree as one more row (nil leaf when detection is disabled).
	resolveToolchain(appConfig, gitter, startRelease, untilTag, description, evidence.Leaf("toolchain"))

	return startRelease, description, nil
}

// resolveToolchain runs toolchain detection (when enabled), drives its row in the evidence tree,
// and attaches the result to the description. Detection is best-effort: any failure is logged and
// does not abort changelog generation. Reconciliation/downgrade warnings are surfaced to the
// operator log here. The leaf is nil (a no-op) when detection is disabled.
func resolveToolchain(appConfig *createConfig, gitter git.Interface, startRelease *release.Release, untilTag string, description *release.Description, leaf *event.Leaf) {
	cfg := appConfig.Toolchain.ToToolchainConfig()
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
		leaf.Resolve("—", "no baseline")
		return
	}

	data, err := toolchain.Detect(gitter, cfg, sinceRef, untilRef)
	if err != nil {
		leaf.Fail(err)
		log.WithFields("error", err).Warn("toolchain detection failed")
		return
	}

	description.Toolchain = data
	count, note := toolchainLeafSummary(data)
	leaf.Resolve(count, note)
	logToolchainWarnings(data)
}

// toolchainLeafSummary renders the toolchain evidence row: the change count in the count column
// (matching the other evidence rows) and a human-readable detail in the trailer note — the version
// transition for a single change, or a downgrade tally for several.
func toolchainLeafSummary(data *release.ToolchainData) (count, note string) {
	if data == nil || len(data.Updates) == 0 {
		return "0", "no change"
	}

	if len(data.Updates) == 1 {
		u := data.Updates[0]
		note = fmt.Sprintf("%s %s → %s", release.ToolLabel(u.Tool), u.From, u.To)
		if u.Direction == release.ToolchainDowngrade {
			note += ", downgrade"
		}
		return "1", note
	}

	downgrades := 0
	for _, u := range data.Updates {
		if u.Direction == release.ToolchainDowngrade {
			downgrades++
		}
	}
	if downgrades > 0 {
		note = fmt.Sprintf("%d downgraded", downgrades)
	}
	return fmt.Sprintf("%d", len(data.Updates)), note
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

	prsKept := summer.PRsKept()
	issuesKept := summer.IssuesKept()
	associatedCommits := summer.AssociatedCommits()

	evidence.Leaf("commits").Resolve(strconv.Itoa(commitTotal),
		droppedTrailer(commitTotal, associatedCommits))
	evidence.Leaf("issues").Resolve(strconv.Itoa(issueTotal),
		droppedTrailer(issueTotal, issuesKept))
	evidence.Leaf("pull requests").Resolve(strconv.Itoa(prTotal),
		droppedTrailer(prTotal, prsKept))
}

// droppedTrailer formats the "(N dropped)" parenthetical, returning empty when
// nothing was dropped (so the row renders without any trailer).
func droppedTrailer(total, kept int) string {
	dropped := total - kept
	if dropped <= 0 {
		return ""
	}
	return fmt.Sprintf("%d dropped", dropped)
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
	switch {
	case sinceTag != "":
		if t, err := gitter.SearchForTag(sinceTag); err == nil && t != nil {
			rng.Slot("since").Resolve(t.Name, shortSha(t.Commit), formatDate(t.Timestamp))
		} else {
			rng.Slot("since").Resolve(sinceTag, formatDate(time.Time{}))
		}
	case desc != nil && desc.PreviousRelease != nil:
		ver := desc.PreviousRelease.Version
		if t, err := gitter.SearchForTag(ver); err == nil && t != nil {
			rng.Slot("since").Resolve(ver, shortSha(t.Commit), formatDate(desc.PreviousRelease.Date))
		} else {
			rng.Slot("since").Resolve(ver, formatDate(desc.PreviousRelease.Date))
		}
	default:
		// no prior release: since the beginning of git history.
		if sha, err := gitter.FirstCommit(); err == nil {
			rng.Slot("since").Resolve(shortSha(sha))
		}
	}

	// until: prefer the resolved tag; otherwise show HEAD.
	if untilTag != "" {
		if t, err := gitter.SearchForTag(untilTag); err == nil && t != nil {
			rng.Slot("until").Resolve(t.Name, shortSha(t.Commit), formatDate(t.Timestamp))
		} else {
			rng.Slot("until").Resolve(untilTag)
		}
	} else if sha, err := gitter.HeadTagOrCommit(); err == nil {
		rng.Slot("until").Resolve(shortSha(sha))
	}
}

// shortSha returns the leading 7 chars of a commit sha (the conventional
// short form), or the input if shorter.
func shortSha(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// formatDate renders a timestamp in the compact form used in the summary
// trailer (e.g. "Jan 15 2026"). Empty time renders as "".
func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 2 2006")
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
