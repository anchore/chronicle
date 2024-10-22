package main

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
	"github.com/anchore/go-make/tasks/release"
)

func main() {
	Makefile(
		RollupTask("default", "run all validations", "static-analysis", "test"),
		golint.Tasks(),
		release.Tasks(),
		fixturesFingerprintTask,
		fixturesTask,
		gotest.Test("unit", gotest.WithDependencies(fixturesTask.Name)),
	)
}

var fixturesTask = Task{
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
	Tasks: []Task{
		{
			Name:   "fixtures:clean",
			Desc:   "clean internal git test fixture caches",
			Labels: All("clean"),
			Run: func() {
				InDir("internal/git/test-fixtures", func() {
					Run("make clean")
				})

				InDir("chronicle/release/releasers/github/test-fixtures", func() {
					Run("make clean")
				})
			},
		},
	},
}

var fixturesFingerprintTask = Task{
	Name:  "fixtures-fingerprint",
	Desc:  "get test fixtures cache fingerprint",
	Quiet: true,
	Run: func() {
		Log(FingerprintGlobs("internal/git/test-fixtures/*.sh"))
	},
}
