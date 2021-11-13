# chronicle

[![Validations](https://github.com/anchore/chronicle/actions/workflows/validations.yaml/badge.svg)](https://github.com/anchore/chronicle/actions/workflows/validations.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/anchore/chronicle)](https://goreportcard.com/report/github.com/anchore/chronicle)
[![GitHub release](https://img.shields.io/github/release/anchore/chronicle.svg)](https://github.com/anchore/chronicle/releases/latest)
[![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/anchore/chronicle.svg)](https://github.com/anchore/chronicle)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/anchore/chronicle/blob/main/LICENSE)
[![Slack Invite](https://img.shields.io/badge/Slack-Join-blue?logo=slack)](https://anchore.com/slack)


A fast changelog generator that sources changes from GitHub PRs and issues, organized by labels.

```bash
# create a changelog from the last GitHib release until the current git HEAD tag/commit
# for the git repo in the current directory
chronicle 

# create a changelog with all changes from v0.16.0 until current git HEAD tag/commit
# for the git repo in the current directory
chronicle --since-tag v0.16.0

# create a changelog between two specific tags for a repo at the given path
chronicle --since-tag v0.16.0 --until-tag v0.18.0 ./path/to/git/repo
```

## Installation

```bash
curl -sSfL https://raw.githubusercontent.com/anchore/chronicle/main/install.sh | sh -s -- -b /usr/local/bin
```

...or, you can specify a release version and destination directory for the installation:

```
curl -sSfL https://raw.githubusercontent.com/anchore/chronicle/main/install.sh | sh -s -- -b <DESTINATION_DIR> <RELEASE_VERSION>
```

