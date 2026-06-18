# chronicle

[![Validations](https://github.com/anchore/chronicle/actions/workflows/validations.yaml/badge.svg)](https://github.com/anchore/chronicle/actions/workflows/validations.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/anchore/chronicle)](https://goreportcard.com/report/github.com/anchore/chronicle)
[![GitHub release](https://img.shields.io/github/release/anchore/chronicle.svg)](https://github.com/anchore/chronicle/releases/latest)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/anchore/chronicle.svg)](https://github.com/anchore/chronicle)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/chronicle/blob/main/LICENSE)
[![Slack Invite](https://img.shields.io/badge/Slack-Join-blue?logo=slack)](https://anchore.com/slack)


**A fast changelog generator that sources changes from GitHub PRs and issues, organized by labels.**


Create a changelog from the last GitHib release until the current git HEAD tag/commit for the git repo in the current directory:
```bash
chronicle 
```

Create a changelog with all changes from v0.16.0 until current git HEAD tag/commit for the git repo in the current directory:
```bash
chronicle --since-tag v0.16.0
```

Create a changelog between two specific tags for a repo at the given path
```bash
chronicle --since-tag v0.16.0 --until-tag v0.18.0 ./path/to/git/repo
```

Create a changelog and guess the release version from the set of changes in the changelog
```bash
chronicle -n
```

Just print the next release version based on the set of changes (don't create a changelog)
```bash
chronicle -o version
```

Write a changelog to a file and the resolved version to another file in one run (nothing on stdout)
```bash
chronicle -o md=CHANGELOG.md -o version=VERSION
```

Render the changelog with ANSI styling for the terminal (falls back to plain markdown if stdout isn't a TTY)
```bash
chronicle -o md-pretty
```

Include a "Toolchain" section that reports minimum-version bumps (e.g. the `go` directive in `go.mod`) between the two changelog points
```bash
chronicle --detect-toolchain
```

## Installation

```bash
curl -sSfL https://get.anchore.io/chronicle | sudo sh -s -- -b /usr/local/bin
```

...or, you can specify a release version and destination directory for the installation:

```
curl -sSfL https://get.anchore.io/chronicle | sudo sh -s -- -b <DESTINATION_DIR> <RELEASE_VERSION>
```

## Configuration

Configuration search paths:
  - `.chronicle.yaml`
  - `.chronicle/config.yaml`
  - `~/.chronicle.yaml`
  - `<XDG_CONFIG_HOME>/chronicle/config.yaml`

### Default values

Configuration options (example values are the default):

```yaml
# output format(s); each entry is NAME or NAME=PATH. Repeat to write more
# than one format/destination in a single run. Available NAMEs:
#   md         — plain markdown
#   md-pretty  — ANSI-styled markdown (stdout only; falls back to md if not a TTY)
#   json       — release description as JSON
#   version    — just the resolved version string with a trailing newline
#   slack      — Slack "mrkdwn" suitable for a webhook payload's text field
# An entry with no path writes to stdout (at most one entry may write to
# stdout).
# same as -o, --output, and CHRONICLE_OUTPUT env var
output:
  - md
  # - md=CHANGELOG.md
  # - version=VERSION
  # - json
  # - md-pretty

# suppress all logging output
# same as -q ; CHRONICLE_QUIET env var
quiet: false

# all logging options
log:
  # use structured logging
  # same as CHRONICLE_LOG_STRUCTURED env var
  structured: false

  # the log level
  # same as CHRONICLE_LOG_LEVEL env var
  level: "warn"

  # location to write the log file (default is not to have a log file)
  # same as CHRONICLE_LOG_FILE env var
  file: ""

# guess what the next release version is based on the current version and set of changes (cannot be used with --until-tag)
# same as --speculate-next-version / -n ; CHRONICLE_SPECULATE_NEXT_VERSION env var
speculate-next-version: false

# override the starting git tag for the changelog (default is to detect the last release automatically)
# same as --since-tag / -s ; CHRONICLE_SINCE_TAG env var
since-tag: ""

# override the ending git tag for the changelog (default is to use the tag or commit at git HEAD)
# same as --until-tag / -u ; CHRONICLE_SINCE_TAG env var
until-tag: ""

# if the current release version is < v1.0 then breaking changes will bump the minor version field
# same as CHRONICLE_ENFORCE_V0 env var
enforce-v0: false

# the title used for the changelog
# same as CHRONICLE_TITLE
title: Changelog

# all github-related settings
github:
  
  # the github host to use (override for github enterprise deployments)
  # same as CHRONICLE_GITHUB_HOST env var
  host: github.com
  
  # do not consider any issues or PRs with any of the given labels
  # same as CHRONICLE_GITHUB_EXCLUDE_LABELS env var
  exclude-labels:
    - duplicate
    - question
    - invalid
    - wontfix
    - wont-fix
    - release-ignore
    - changelog-ignore
    - ignore
  
  # consider merged PRs as candidate changelog entries (must have a matching label from a 'github.changes' entry)
  # same as CHRONICLE_GITHUB_INCLUDE_PRS env var
  include-prs: true

  # consider closed issues as candidate changelog entries (must have a matching label from a 'github.changes' entry)
  # same as CHRONICLE_GITHUB_INCLUDE_ISSUES env var
  include-issues: true

  # issues can only be considered for changelog candidates if they have linked PRs that are merged (note: does NOT require github.include-issues to be set)
  # same as CHRONICLE_GITHUB_ISSUES_REQUIRE_LINKED_PRS env var
  issues-require-linked-prs: false

  # when a merged PR has no change-type label, infer the change type from a conventional-commit
  # prefix in the PR title (e.g. "feat: ..." -> Added Features). An explicit label always wins.
  # same as CHRONICLE_GITHUB_INFER_CHANGE_TYPE_FROM_TITLE env var
  infer-change-type-from-title: true

  # list of definitions of what labels applied to issues or PRs constitute a changelog entry. These entries also dictate 
  # the changelog section, the changelog title, and the semver field that best represents the class of change.
  # note: cannot be set via environment variables
  changes: [...<list of entries>...] # See "Default GitHub change definitions" section for more details

# detect toolchain-requirement bumps (e.g. the minimum Go version) between the changelog points. Opt-in.
# See the "Toolchain detection" section for more details.
toolchain:

  # master switch; when false no toolchain detection is performed
  # same as --detect-toolchain ; CHRONICLE_TOOLCHAIN_ENABLED env var
  enabled: false

  # which ecosystems to inspect; empty means all known (currently: go)
  # same as --toolchain-ecosystems ; CHRONICLE_TOOLCHAIN_ECOSYSTEMS env var
  ecosystems: []

  # path globs excluded from source-file discovery (keeps vendored/test manifests out by default)
  # same as CHRONICLE_TOOLCHAIN_IGNORE env var
  ignore:
    - '**/vendor/**'
    - '**/node_modules/**'
    - '**/testdata/**'
    - '**/examples/**'

  # per-ecosystem source-file globs (override to narrow, e.g. ['./go.mod'], or widen)
  go:
    paths:
      - '**/go.mod'

```

### Default GitHub change definitions

The `github.changes` configurable is a list of mappings, each that take the following fields:

- `name`: _[string]_ singular, lowercase, hyphen-separated (no spaces) name that best represents the change (e.g. "breaking-change", "security", "added-feature", "enhancement", "new-feature", etc).
- `title`: _[string]_ title of the section in the changelog listing all entries.
- `semver-field`: _[string]_ change entries will bump the respective semver field when guessing the next release version. Allowable values: `major`, `minor`, or `patch`.
- `labels`: _[list of strings]_ all issue or PR labels that should match this change section.
- `prefixes`: _[list of strings]_ [conventional-commit](https://www.conventionalcommits.org/en/v1.0.0/#specification) type prefixes that map to this change section when `github.infer-change-type-from-title` is enabled and a PR carries no change-type label (e.g. `feat`, `fix`). Prefixes are matched case-insensitively. Use the special value `!` to match the conventional-commit breaking-change marker (e.g. a `feat!: ...` title); if a PR title carries a `!` marker but no change section declares the `!` prefix, chronicle logs a warning and falls back to the base type (so the breaking change won't bump the major version). Only applies to PRs, not issues; an explicit label always takes precedence over an inferred prefix.

The default value for `github.changes` is:

```yaml
- name: security-fixes
  title: Security Fixes
  semver-field: patch
  labels:
    - security
    - vulnerability
  
- name: added-feature
  title: Added Features
  semver-field: minor
  labels:
    - enhancement
    - feature
    - minor
  prefixes:
    - feat
  
- name: bug-fix
  title: Bug Fixes
  semver-field: patch
  labels:
    - bug
    - fix
    - bug-fix
    - patch
  prefixes:
    - fix

- name: performance
  title: Performance
  semver-field: patch
  labels:
    - performance
    - perf
  prefixes:
    - perf
  
- name: breaking-feature
  title: Breaking Changes
  semver-field: major
  labels:
    - breaking
    - backwards-incompatible
    - breaking-change
    - breaking-feature
    - major
  prefixes:
    - "!"
    
- name: removed-feature
  title: Removed Features
  semver-field: major
  labels:
    - removed
  
- name: deprecated-feature
  title: Deprecated Features
  semver-field: minor
  labels:
    - deprecated

- name: unknown
  title: Additional Changes
```

## Dependency scanning

Chronicle can diff the dependency graph between the `since` and `until` refs and render the results as a `### Dependencies` section in the changelog. Each changed package is reported as added, updated, downgraded, or removed. With vulnerability annotation enabled, chronicle also notes which CVEs/GHSAs were remediated or introduced by each change.

This feature is **opt-in**: it runs only when you request one or more ecosystems.

### Enabling

Pass `--dependencies` with the ecosystem(s) to scan. The ecosystem value is a [syft cataloger selection](https://github.com/anchore/syft) expression — `language` (all language ecosystems), or a specific one like `go`, `python`, `javascript`, `java`, `ruby`, `rust`, `php`, `dotnet`. The flag is repeatable and accepts comma-separated values:

```bash
chronicle --dependencies go
chronicle --dependencies go,python
chronicle --dependencies language
```

Scoping to an ecosystem keeps the section focused: `--dependencies go` reports only Go modules, not the GitHub Actions, OS packages, or other manifests syft can also find in the tree.

Use `auto` to detect ecosystems from manifests at the repository root and enable only those — handy for a single config that works across repos. The markers come straight from syft's own cataloger metadata, so detection matches what syft would actually catalog: it keys off the manifests/lockfiles syft's declared-language catalogers read (e.g. `go.mod` → `go`, `package-lock.json`/`yarn.lock` → `javascript`, `Cargo.lock` → `rust`), not every file an ecosystem might have. Detection is root-only; if nothing is recognized the section is simply omitted. `auto` may be combined with explicit ecosystems (`--dependencies auto,go`).

```bash
chronicle --dependencies auto
```

To disable the feature — for example to override an `auto` set in a committed config for one run — pass `none`, which wins over every other value:

```bash
chronicle --dependencies none
```

To annotate each change with the CVEs/GHSAs it remediated or introduced, add `--vulnerabilities` (this downloads/loads the grype vulnerability DB — see below):

```bash
chronicle --dependencies go --vulnerabilities
```

### Example output

By default each change kind is **collapsed** — a count that expands to a compact
list — so the section stays small (this renders as a `<details>` block on
GitHub). Each entry is the package name, the version transition (long Go
pseudo-versions are shortened to a 7-char commit hash), and any vulnerability
note in italics:

````
### Dependencies

23 dependency changes (12 updated, 8 added, 3 removed). 47 vulnerabilities remediated.

<details>
<summary>Updated (12 packages)</summary>

- **golang.org/x/net** `v0.17.0 → v0.23.0` *(remediated CVE-2023-44487, CVE-2023-39325)*
- **github.com/google/pprof** `v0.0.0-6f57359 → v0.0.0-6e76a2b`
- **github.com/foo/bar** `v1.2.0 → v1.1.0` *(⚠ reintroduces CVE-2024-12345)*
</details>

<details>
<summary>Added (8 packages)</summary>

- **github.com/new/dep** `v0.4.0`
</details>
````

Set an action to `list` for an always-expanded list, or `summary` for a count
only. When more than one ecosystem is present, changes are grouped into
`#### Go`, `#### Python`, … subsections under the `### Dependencies` header.
Vulnerability notes appear only when `--vulnerabilities` is set.

### Configuration

All dependency options live under the `dependencies:` key. Default values are shown:

```yaml
dependencies:
  # ecosystems to scan (syft cataloger selection, e.g. language, go, python).
  # the feature is enabled when this is non-empty. "language" selects all
  # language ecosystems. "auto" detects ecosystems from root manifests; "none"
  # disables the feature and wins over all other values. same as --dependencies
  # (repeatable / comma-separated).
  ecosystems: []

  # paths to exclude from the scan (e.g. vendored or test trees), as syft
  # exclude patterns. each pattern must start with "./", "*/", or "**/" and is
  # matched relative to the repo root — e.g. ./vendor, **/testdata, */examples.
  exclude: []

  # annotate dependency changes with known vulnerability information
  # same as --vulnerabilities. requires at least one ecosystem above (there is
  # nothing to annotate without a dependency diff).
  annotate-vulnerabilities: false

  # only show dependency changes that remediated or introduced a vulnerability
  # (a security-focused view). requires annotate-vulnerabilities; no effect
  # without it. (config-only, no flag)
  only-vulnerable: false

  # show the remaining (carried-over) vulnerabilities still present in the latest
  # scan that this release did not remediate, as a "🟡 Remaining" rollup. unlike
  # the remediated/introduced rollups (which cover only changed packages), this
  # spans every package in the latest scan, so it is the standing burden rather
  # than the release's delta. requires annotate-vulnerabilities; no effect
  # without it. (config-only, no flag)
  show-remaining-vulnerabilities: false

  # minimum vulnerability severity to include in annotations (e.g. low, medium, high, critical)
  # empty string includes all severities (config-only, no flag)
  min-severity: ""

  # detect declared toolchain minimum-version changes (e.g. the go directive in
  # go.mod) for the activated ecosystems, shown as a Toolchains rollup under
  # Dependencies. on by default; set false to disable. (config-only, no flag)
  detect-toolchain: true

  # how each change kind is displayed. each value is a comma-separated list of
  # fallback modes; the encoder uses the first one it supports. modes:
  #   hide      - omit the kind
  #   summary   - count only, e.g. "Added (20 packages)"
  #   list      - a full bullet list (markdown and slack)
  #   collapsed - a bullet list inside a <details> block (markdown/GitHub only)
  # e.g. "collapsed,list" collapses in markdown but enumerates in slack (which
  # cannot collapse). a bare "collapsed" degrades to "list" where unsupported.
  actions:
    updated:    collapsed,list
    downgraded: collapsed,list
    added:      collapsed,list
    removed:    collapsed,list
```

When `only-vulnerable` is active the per-kind headers note that the count is the
vuln-affected subset — e.g. `Updated (11 packages with vulnerability changes)` —
so it is clearly distinct from the full totals in the summary line. Each listed
package's vulnerability note shows the 🟢-flagged remediated IDs and any
🔴-flagged (re)introduced IDs, with each ID linked to its data source.

When `show-remaining-vulnerabilities` is active a `🟡 Remaining` rollup lists
the carried-over
vulnerabilities — those present at both endpoints, on any package in the latest
scan — that the release did not clear. It complements the remediated/introduced
rollups (which only cover changed packages) with the standing burden that
remains. In markdown it appears under the Dependencies heading whether or not
the change lists are collapsed (it has no per-package inline equivalent); in
slack it is the only vulnerability rollup shown.

### Vulnerability database

When `annotate-vulnerabilities` is enabled, chronicle uses grype's vulnerability database. The DB is stored in grype's default cache directory (`~/.cache/grype/db/...`), so if you already use the grype CLI the cache is shared between them.

- **First run**: the DB is downloaded on demand (hundreds of MB) and requires network access.
- **Subsequent runs**: chronicle reads from the local cache and checks for DB updates on each run (matching grype's default behavior).

### Limitations

- **Source/declared dependencies only.** The scan reads `go.mod`, lockfiles, and vendored manifests — it does not see base-image or OS packages. A base-image bump that removes OS-level CVEs will not appear in this section.
- **First-run download cost.** The initial DB download is several hundred megabytes and requires network access; subsequent runs use the local grype cache.
- **Annotation adds runtime cost.** Enabling `annotate-vulnerabilities` loads the vulnerability DB and runs two grype match passes (one for each ref), which increases wall-clock time compared to a plain dependency diff.

## Toolchain detection

Toolchain detection rides on the dependency scanning feature: whenever `--dependencies` is active it also reports changes to a project's declared minimum toolchain version between the changelog's start and end points, for the same ecosystems being scanned. Today this covers Go: the `go` directive in `go.mod` (the `toolchain` directive — a pinned toolchain — is intentionally ignored, since only the declared minimum is reported). It is on by default and can be turned off with `dependencies.detect-toolchain: false`.

A detected bump renders as a **Toolchains** rollup inside the `### Dependencies` section, alongside the vulnerability rollups:

```markdown
### Dependencies

...

**Toolchains (1)**

- Go minimum version: 1.21 → 1.23
```

Detection only covers the **activated dependency ecosystems** — `--dependencies go` (or `language`) inspects Go's `go.mod`, while an ecosystem with no toolchain detector (e.g. `--dependencies java`) contributes nothing. When the dependency diff finds no package changes but a toolchain bump did occur, the `### Dependencies` section still renders with just the Toolchains rollup.

Downgrades (a minimum version moving *backward*) are called out explicitly, since they're usually unintentional — both inline in the rollup and as an operator log warning:

```markdown
**Toolchains (1)**

- Go minimum version: 1.23 → 1.21 (downgrade)
```

Detection is **source-only**: chronicle reads the relevant manifest files directly from git at each ref (no checkout, build, or install) and compares the declared minimums. Discovery walks all matching files (e.g. `**/go.mod`) minus the default ignore globs and the dependencies `exclude` list, so multi-module repos are covered by default while vendored and test manifests stay out. When sources within one ecosystem disagree on the resulting version, chronicle warns (to the log and JSON output) but still emits the changelog — it degrades gracefully rather than failing.

### Why not use Syft for this?

Syft answers a different question — it inventories the *packages a project depends on*, whereas toolchain detection needs the *minimum toolchain version a project declares it requires*, which is a single declarative field in a source file. Pulling in Syft would mean materializing and scanning whole trees at two refs just to diff one line of `go.mod`, so chronicle reads that field straight from git instead.
