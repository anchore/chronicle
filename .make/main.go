package main

import (
	"fmt"
	"strings"

	. "github.com/anchore/go-make"
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
		verifyGeneratedTask(),
	)
}

// generatedFiles are the checked-in artifacts produced by `go generate`. They are
// idempotent, so a clean checkout that re-runs generation must see no changes.
var generatedFiles = []string{
	"cmd/chronicle/cli/options/ecosystems.gen.yaml",
}

// verifyGeneratedTask re-runs code generation and fails if a committed artifact
// drifted — guarding the syft-derived ecosystem detection table against a stale
// check-in (e.g. a syft bump landed without re-running `go generate`). It hooks
// into static-analysis so CI enforces it on every PR.
func verifyGeneratedTask() Task {
	return Task{
		Name:        "verify-generated",
		Description: "ensure committed generated files match `go generate` output",
		RunsOn:      lang.List("static-analysis"),
		Run: func() {
			Run("go generate ./cmd/chronicle/cli/options/...")
			args := strings.Join(generatedFiles, " ")
			if dirty := strings.TrimSpace(Run("git status --porcelain -- "+args, run.NoFail())); dirty != "" {
				lang.Throw(fmt.Errorf("generated files are out of date; run `go generate ./...` and commit:\n%s", dirty))
			}
		},
	}
}
