package main

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
	"github.com/anchore/go-make/tasks/release"
)

func main() {
	fixturesTask := Task{
		Name: "fixtures",
		Desc: "build test fixtures",
		Run: func() {
			InDir("internal/git/test-fixtures", func() {
				Run("make")
			})

			InDir("chronicle/release/releasers/github/test-fixtures", func() {
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
		golint.Tasks(),
		release.Tasks(),
		fixturesFingerprintTask,
		fixturesTask,
		gotest.Test("unit"),
	)
}
