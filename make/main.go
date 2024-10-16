package main

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/tasks/gobuild"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
	"github.com/anchore/go-make/tasks/release"
)

func main() {
	fixturesTask := Task{
		Name: "fixtures",
		Desc: "build test fixtures",
		Run: func() {
			PushPopd("internal/git/test-fixtures", func() {
				Run("make")
			})

			PushPopd("chronicle/release/releasers/github/test-fixtures", func() {
				Run("make")
			})
		},
	}

	fixturesFingerprintTask := Task{
		Name:  "fixtures-fingerprint",
		Desc:  "get test fixtures cache fingerprint",
		Quiet: true,
		Run: func() {
			Log(FingerprintGlobs("internal/git/test-fixtures/*.sh"))
		},
	}

	Makefile(
		RollupTask("default", "run all validations", "static-analysis", "test"),
		golint.FormatTask(),
		golint.LintFixTask(),
		golint.StaticAnalysisTask(),
		release.ChangelogTask(),
		release.WorkflowTask(),
		release.CIReleaseTask(),
		gobuild.SnapshotTask(),
		fixturesFingerprintTask,
		fixturesTask,
		gotest.Test("unit"),
		RollupTask("test", "run all levels of test", "unit"),
	)
}

// TODO: clean
// TODO: there is a task with task children that also get registered
// TODO: there are task labels that also act as names
