package main

import (
	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/gotest"
	"github.com/anchore/go-make/tasks/release"
)

func main() {
	Makefile(
		golint.Tasks(),
		release.Tasks(),
		gotest.Tasks(),
		gotest.FixtureTasks().RunOn("release"),
	)
}
