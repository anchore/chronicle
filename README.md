# chronicle

A fast changelog generator that sources changes from GitHub PRs and issues, organized by labels.

```bash
chronicle --since-tag v0.16.0
chronicle --since-tag v0.16.0 --until-tag v0.18.0
```

TODO:
- include PRs
- consider PRs with labels
- Add exclusion by label(s) (both PR and issue)
- Add merged PRs section (that don't have labels that allow change type selection)
- Add closed issue section (that don't have labels that allow change type selection)
- Chain set of changelogs from previous release changelogs (pull from releases MD for each)
- Markdown: add assignee(s) name + url
- Repo extraction: extract from multiple URL sources (not just git, e.g. git@github.com:someone/project.git... should at least support https)
- Support rate limit detection and retry