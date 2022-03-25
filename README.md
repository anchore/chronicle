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
chronicle --speculate-next-version
```

Just guess the next release version based on the set of changes (don't create a changelog)
```bash
chronicle next-version
```

## Installation

```bash
curl -sSfL https://raw.githubusercontent.com/anchore/chronicle/main/install.sh | sh -s -- -b /usr/local/bin
```

...or, you can specify a release version and destination directory for the installation:

```
curl -sSfL https://raw.githubusercontent.com/anchore/chronicle/main/install.sh | sh -s -- -b <DESTINATION_DIR> <RELEASE_VERSION>
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
# the output format of the changelog
# same as -o, --output, and CHRONICLE_OUTPUT env var
output: md

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
```
