# Developing

## Getting started

In order to test and develop in this repo you will need the following dependencies installed:
- make

After cloning do the following:
1. run `make bootstrap` to download go mod dependencies, create the `/.tmp` dir, and download helper utilities.
2. run `make` to run linting, tests, and other verifications to make certain everything is working alright.

Checkout `make help` to see what other actions you can take.

The main make tasks for common static analysis and testing are `lint`, `format`, `lint-fix`, and `unit`.

## Architecture

At the highest level chronicle creates a changelog based off of a source repo. This is done by the following flow:

```text
since tag -> release.ChangelogInfo(...) -> release.Description -> presenter.Present(io.Writer)
until tag ->
repo path ->
...       ->
```

The only support source to generate changelogs from at the time of this writing is GitHub, which enables chronicle 
to use GitHub releases, issues, and PRs to figure the contents of the next changelog. There are a few abstractions 
that this functionality has been implemented behind so that future support for other sources will be easier (e.g. GitLab):

- `release.Summarizer` : This is meant to be the interface between the application and the source for all release information. Allows one to get information about the previous releases, the changes between two VCS references, and supporting URLs that one can visit to learn more.  Implementing this is the first step to adding a new source. For the `github.Summarizer`, most of the work is done within `github.Summarizer.Changes()` fetching all issues and PRs and filtering down by date and label.

- `release.VersionSpeculator` : an object that knows how to figure the next release version given the current release and a set of changes.

In the `cmd` package, a worker that encapsulates creating the correct implementation of these abstractions for the detected or configured source are instantiated and passed to the common `release.ChangelogInfo` helper which returns a static description of all of the information necessary for changelog presentation. 

As of today chronicle supports outputting this description as either `json` or `markdown` which can be found in the `chronicle/format` package.