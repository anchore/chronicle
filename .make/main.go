package main

import (
	"fmt"
	"strings"

	. "github.com/anchore/go-make"
	"github.com/anchore/go-make/file"
	"github.com/anchore/go-make/lang"
	"github.com/anchore/go-make/run"
	"github.com/anchore/go-make/tasks/golint"
	"github.com/anchore/go-make/tasks/goreleaser"
	"github.com/anchore/go-make/tasks/gotest"
)

func main() {
	Makefile(
		golint.Tasks(),
		goreleaser.Tasks(),
		gotest.Tasks(),
		gotest.FixtureTasks().RunOn("unit"),
		generateTask(),
		verifyGeneratedTask(),
		updateSyftTask(),
		installTestTasks(),
	)
}

// generatedFiles are the checked-in artifacts produced by `go generate`. They are
// idempotent, so a clean checkout that re-runs generation must see no changes.
var generatedFiles = []string{
	"cmd/chronicle/cli/options/ecosystems.gen.yaml",
}

// generate runs code generation for the syft-derived ecosystem detection table.
// Shared by generateTask and updateSyftTask so the two can't drift apart.
func generate() {
	Run("go generate ./cmd/chronicle/cli/options/...")
}

// generateTask regenerates the syft-derived ecosystem detection table. Run it
// after bumping syft; `update-syft` does both.
func generateTask() Task {
	return Task{
		Name:        "generate",
		Description: "regenerate ecosystem detection artifacts via `go generate`",
		Run:         generate,
	}
}

// updateSyftTask bumps syft to its latest release and regenerates the artifacts
// derived from it. A weekly job runs it so the bump and the regenerated file land
// together, ahead of Dependabot's syft bump (which can't regenerate and so fails
// verify-generated). Generation runs after the bump, so this can't depend on
// `generate` (dependencies run first).
func updateSyftTask() Task {
	return Task{
		Name:        "update-syft",
		Description: "bump syft to the latest release and regenerate derived artifacts",
		Run: func() {
			Run("go get github.com/anchore/syft@latest")
			Run("go mod tidy")
			generate()
		},
	}
}

// verifyGeneratedTask re-runs code generation and fails if a committed artifact
// drifted — guarding the syft-derived ecosystem detection table against a stale
// check-in (e.g. a syft bump landed without re-running `go generate`). It hooks
// into static-analysis so CI enforces it on every PR.
func verifyGeneratedTask() Task {
	return Task{
		Name:         "verify-generated",
		Description:  "ensure committed generated files match `go generate` output",
		Dependencies: Deps("generate"),
		RunsOn:       lang.List("static-analysis"),
		Run: func() {
			args := strings.Join(generatedFiles, " ")
			if dirty := strings.TrimSpace(Run("git status --porcelain -- "+args, run.NoFail())); dirty != "" {
				lang.Throw(fmt.Errorf("generated files are out of date; run `go generate ./...` and commit:\n%s", dirty))
			}
		},
	}
}

// installTestTasks runs install.sh tests inside docker containers (driven by test/install/Makefile).
func installTestTasks() Task {
	inDir := func(target string) func() {
		return func() { file.InDir("test/install", func() { Run("make " + target) }) }
	}
	return Task{
		Tasks: []Task{
			{Name: "install-test", Description: "run install.sh unit and acceptance tests", Run: inDir("test")},
			{Name: "install-test-cache-save", Description: "save install.sh test image cache", Run: inDir("save")},
			{Name: "install-test-cache-load", Description: "load install.sh test image cache", Run: inDir("load")},
			{Name: "install-test-ci-mac", Description: "run install.sh tests on mac (CI)", Run: inDir("ci-test-mac")},
		},
	}
}
