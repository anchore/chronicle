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
  
- name: bug-fix
  title: Bug Fixes
  semver-field: patch
  labels:
    - bug
    - fix
    - bug-fix
    - patch
  
- name: breaking-feature
  title: Breaking Changes
  semver-field: major
  labels:
    - breaking
    - backwards-incompatible
    - breaking-change
    - breaking-feature
    - major
    
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

## Toolchain detection

When enabled (`--detect-toolchain` or `toolchain.enabled: true`), chronicle reports changes to a project's declared minimum toolchain version between the changelog's start and end points. Today this covers Go: the `go` directive in `go.mod` (the `toolchain` directive — a pinned toolchain — is intentionally ignored, since only the declared minimum is reported). A detected bump renders as its own section:

```markdown
### Toolchain

- Go: 1.21 → 1.23
```

Downgrades (a minimum version moving *backward*) are called out explicitly, since they're usually unintentional — both inline in the section and as an operator log warning:

```markdown
### Toolchain

- Go: 1.23 → 1.21 (downgrade)
```

Detection is **opt-in** and **source-only**: chronicle reads the relevant manifest files directly from git at each ref (no checkout, build, or install) and compares the declared minimums. Discovery walks all matching files (e.g. `**/go.mod`) minus the `ignore` globs, so multi-module repos are covered by default while vendored and test manifests stay out; narrow `go.paths` to something like `['./go.mod']` if you want only the root module. When sources within one ecosystem disagree on the resulting version, chronicle warns (to the log and JSON output) but still emits the changelog — it degrades gracefully rather than failing.

### Why not use Syft for this?

Syft answers a different question — it inventories the *packages a project depends on*, whereas toolchain detection needs the *minimum toolchain version a project declares it requires*, which is a single declarative field in a source file. Pulling in Syft would mean materializing and scanning whole trees at two refs just to diff one line of `go.mod`, so chronicle reads that field straight from git instead.
