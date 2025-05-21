package main

import (
	"path/filepath"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
	"github.com/anchore/go-make/tasks/release"
)

func main() {
	Makefile(
		RollupTask("default", "run all validations", "static-analysis", "test"),
		golint.Tasks(),
		release.Tasks().DependsOn(fixturesTasks.Name),
		gotest.FixturesTasks(),
		gotest.Test("unit", gotest.WithDependencies(fixturesTasks.Name)).DependsOn(gotest.FixturesTas),
	)
}
