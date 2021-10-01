# chronicle

A fast changelog generator that sources changes from GitHub PRs and issues, organized by labels.

```bash
chronicle --since-tag v0.16.0
chronicle --since-tag v0.16.0 --until-tag v0.18.0
```

TODO:
- [x] include PRs
- [x] Add exclusion by label(s) (both PR and issue)
- [ ] Add merged PRs section (that don't have labels that allow change type selection)
- [ ] Add closed issue section (that don't have labels that allow change type selection)
- [ ] Chain set of changelogs from previous release changelogs (pull from releases MD for each... separate command)
- [ ] Markdown: add assignee(s) name + url (issues)
- [ ] Repo extraction: extract from multiple URL sources (not just git, e.g. git@github.com:someone/project.git... should at least support https)
- [ ] Support rate limit detection and retry
- [x] Merged PRs linked to closed PRs: don't show the change for the PR, show the issue

Questions:
- For linked issues/prs. If there is a merged PR linked to an issue that isn't closed, what do we do?
