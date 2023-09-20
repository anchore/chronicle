BIN = chronicle
TEMP_DIR = ./.tmp
RESULTS_DIR = test/results
COVER_REPORT = $(RESULTS_DIR)/unit-coverage-details.txt
COVER_TOTAL = $(RESULTS_DIR)/unit-coverage-summary.txt

# Command templates #################################
LINT_CMD = $(TEMP_DIR)/golangci-lint run --tests=false --timeout=2m --config .golangci.yaml
GOIMPORTS_CMD = $(TEMP_DIR)/gosimports -local github.com/anchore
RELEASE_CMD = $(TEMP_DIR)/goreleaser release --rm-dist
SNAPSHOT_CMD = $(RELEASE_CMD) --skip-publish --snapshot --skip-sign
CHRONICLE_CMD = $(TEMP_DIR)/chronicle
GLOW_CMD = $(TEMP_DIR)/glow

# Tool versions #################################
GOLANG_CI_VERSION = v1.54.2
GOBOUNCER_VERSION = v0.4.0
GORELEASER_VERSION = v1.17.0
GOSIMPORTS_VERSION = v0.3.8
GLOW_VERSION := v1.5.1

# Formatting variables #################################
BOLD := $(shell tput -T linux bold)
PURPLE := $(shell tput -T linux setaf 5)
GREEN := $(shell tput -T linux setaf 2)
CYAN := $(shell tput -T linux setaf 6)
RED := $(shell tput -T linux setaf 1)
RESET := $(shell tput -T linux sgr0)
TITLE := $(BOLD)$(PURPLE)
SUCCESS := $(BOLD)$(GREEN)

# Test variables #################################
# the quality gate lower threshold for unit test total % coverage (by function statements)
COVERAGE_THRESHOLD := 50
# CI cache busting values; change these if you want CI to not use previous stored cache
FIXTURE_CACHE_BUSTER = "88738d2f"

## Build variables
DIST_DIR=./dist
SNAPSHOT_DIR=./snapshot
OS=$(shell uname | tr '[:upper:]' '[:lower:]')
CHANGELOG := CHANGELOG.md

ifeq ($(OS),Darwin)
	SNAPSHOT_CMD=$(realpath $(shell pwd)/$(SNAPSHOT_DIR)/$(BIN)-macos_darwin_amd64/$(BIN))
else
	SNAPSHOT_CMD=$(realpath $(shell pwd)/$(SNAPSHOT_DIR)/$(BIN)_linux_amd64/$(BIN))
endif

ifeq "$(strip $(VERSION))" ""
 override VERSION = $(shell git describe --always --tags --dirty)
endif

## Variable assertions

ifndef TEMP_DIR
	$(error TEMP_DIR is not set)
endif

ifndef RESULTS_DIR
	$(error RESULTS_DIR is not set)
endif

ifndef DIST_DIR
	$(error DIST_DIR is not set)
endif

ifndef SNAPSHOT_DIR
	$(error SNAPSHOT_DIR is not set)
endif

ifndef REF_NAME
	REF_NAME = $(VERSION)
endif

define title
    @printf '$(TITLE)$(1)$(RESET)\n'
endef

## Tasks

.PHONY: all
all: clean static-analysis test ## Run all linux-based checks
	@printf '$(SUCCESS)All checks pass!$(RESET)\n'

.PHONY: test
test: unit  ## Run all tests


## Bootstrapping targets #################################

.PHONY: ci-bootstrap
ci-bootstrap:
	DEBIAN_FRONTEND=noninteractive sudo apt update && sudo -E apt install -y bc jq libxml2-utils

$(RESULTS_DIR):
	mkdir -p $(RESULTS_DIR)

$(TEMP_DIR):
	mkdir -p $(TEMP_DIR)

.PHONY: bootstrap-tools
bootstrap-tools: $(TEMP_DIR)
	GO111MODULE=off GOBIN=$(realpath $(TEMP_DIR)) go get -u golang.org/x/perf/cmd/benchstat
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(TEMP_DIR)/ $(GOLANG_CI_VERSION)
	curl -sSfL https://raw.githubusercontent.com/wagoodman/go-bouncer/master/bouncer.sh | sh -s -- -b $(TEMP_DIR)/ $(GOBOUNCER_VERSION)
	# we purposefully use the latest version of chronicle released
	#curl -sSfL https://raw.githubusercontent.com/anchore/chronicle/main/install.sh | sh -s -- -b $(TEMP_DIR)/ $(CHRONICLE_VERSION)
	GOBIN="$(realpath $(TEMP_DIR))" go install ./cmd/chronicle
	GOBIN="$(realpath $(TEMP_DIR))" go install github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)
	# the only difference between goimports and gosimports is that gosimports removes extra whitespace between import blocks (see https://github.com/golang/go/issues/20818)
	GOBIN="$(realpath $(TEMP_DIR))" go install github.com/rinchsan/gosimports/cmd/gosimports@$(GOSIMPORTS_VERSION)
	GOBIN="$(realpath $(TEMP_DIR))" go install github.com/charmbracelet/glow@$(GLOW_VERSION)

.PHONY: bootstrap-go
bootstrap-go:
	go mod download

.PHONY: bootstrap
bootstrap: $(RESULTS_DIR) bootstrap-go bootstrap-tools ## Download and install all go dependencies (+ prep tooling in the ./tmp dir)
	$(call title,Bootstrapping dependencies)


## Static analysis targets #################################

.PHONY: static-analysis
static-analysis: lint check-go-mod-tidy check-licenses

.PHONY: lint
lint: ## Run gofmt + golangci lint checks
	$(call title,Running linters)
	# ensure there are no go fmt differences
	@printf "files with gofmt issues: [$(shell gofmt -l -s .)]\n"
	@test -z "$(shell gofmt -l -s .)"

	# run all golangci-lint rules
	$(LINT_CMD)
	@[ -z "$(shell $(GOIMPORTS_CMD) -d .)" ] || (echo "goimports needs to be fixed" && false)

	# go tooling does not play well with certain filename characters, ensure the common cases don't result in future "go get" failures
	$(eval MALFORMED_FILENAMES := $(shell find . | grep -e ':'))
	@bash -c "[[ '$(MALFORMED_FILENAMES)' == '' ]] || (printf '\nfound unsupported filename characters:\n$(MALFORMED_FILENAMES)\n\n' && false)"

.PHONY: format
format: ## Auto-format all source code
	$(call title,Running formatters)
	gofmt -w -s .
	$(GOIMPORTS_CMD) -w .
	go mod tidy

.PHONY: lint-fix
lint-fix: format  ## Auto-format all source code + run golangci lint fixers
	$(call title,Running lint fixers)
	$(LINT_CMD) --fix

.PHONY: check-licenses
check-licenses:
	$(TEMP_DIR)/bouncer check ./...

check-go-mod-tidy:
	@ .github/scripts/go-mod-tidy-check.sh && echo "go.mod and go.sum are tidy!"


## Testing targets #################################

.PHONY: unit
unit: $(RESULTS_DIR) fixtures ## Run unit tests (with coverage)
	$(call title,Running unit tests)
	go test  -coverprofile $(COVER_REPORT) $(shell go list ./... | grep -v anchore/chronicle/test)
	@go tool cover -func $(COVER_REPORT) | grep total |  awk '{print substr($$3, 1, length($$3)-1)}' > $(COVER_TOTAL)
	@echo "Coverage: $$(cat $(COVER_TOTAL))"
	@if [ $$(echo "$$(cat $(COVER_TOTAL)) >= $(COVERAGE_THRESHOLD)" | bc -l) -ne 1 ]; then echo "$(RED)$(BOLD)Failed coverage quality gate (> $(COVERAGE_THRESHOLD)%)$(RESET)" && false; fi


## Test-fixture-related targets #################################

.PHONY: fixtures
fixtures:
	$(call title,Generating test fixtures)
	cd internal/git/test-fixtures && make
	cd chronicle/release/releasers/github/test-fixtures && make

fixtures-fingerprint:
	find internal/git/test-fixtures/*.sh -type f -exec md5sum {} + | awk '{print $1}' | sort | md5sum | tee internal/git/test-fixtures/cache.fingerprint && echo "$(FIXTURE_CACHE_BUSTER)" >> internal/git/test-fixtures/cache.fingerprint


## Build-related targets #################################

.PHONY: build
build: $(SNAPSHOT_DIR) ## Build release snapshot binaries and packages

$(SNAPSHOT_DIR): ## Build snapshot release binaries and packages
	$(call title,Building snapshot artifacts)
	# create a config with the dist dir overridden
	echo "dist: $(SNAPSHOT_DIR)" > $(TEMP_DIR)/goreleaser.yaml
	cat .goreleaser.yaml >> $(TEMP_DIR)/goreleaser.yaml

	# build release snapshots
	BUILD_GIT_TREE_STATE=$(GITTREESTATE) \
	$(TEMP_DIR)/goreleaser build --snapshot --skip-validate --rm-dist --config $(TEMP_DIR)/goreleaser.yaml

.PHONY: changelog
changelog: clean-changelog  ## Generate and show the changelog for the current unreleased version
	$(CHRONICLE_CMD) -vvv -n --version-file VERSION > $(CHANGELOG)
	@$(GLOW_CMD) $(CHANGELOG)

$(CHANGELOG):
	$(CHRONICLE_CMD) -vvv > $(CHANGELOG)

.PHONY: release
release:
	@.github/scripts/trigger-release.sh

.PHONY: ci-release
ci-release: ci-check clean-dist $(CHANGELOG)
	$(call title,Publishing release artifacts)

	# create a config with the dist dir overridden
	echo "dist: $(DIST_DIR)" > $(TEMP_DIR)/goreleaser.yaml
	cat .goreleaser.yaml >> $(TEMP_DIR)/goreleaser.yaml

	bash -c "\
		$(RELEASE_CMD) \
			--config $(TEMP_DIR)/goreleaser.yaml \
			--release-notes <(cat $(CHANGELOG)) \
				 || (cat /tmp/quill-*.log && false)"

	# upload the version file that supports the application version update check (excluding pre-releases)
	.github/scripts/update-version-file.sh "$(DIST_DIR)" "$(VERSION)"


## Cleanup targets #################################

.PHONY: ci-check
ci-check:
	@.github/scripts/ci-check.sh

.PHONY: clean
clean: clean-dist clean-snapshot  ## Remove previous builds, result reports, and test cache
	rm -rf $(RESULTS_DIR)/*

.PHONY: clean-snapshot
clean-snapshot:
	rm -rf $(SNAPSHOT_DIR) $(TEMP_DIR)/goreleaser.yaml

.PHONY: clean-dist
clean-dist: clean-changelog
	rm -rf $(DIST_DIR) $(TEMP_DIR)/goreleaser.yaml

.PHONY: clean-changelog
clean-changelog:
	rm -f $(CHANGELOG)


## Halp! #################################

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(BOLD)$(CYAN)%-25s$(RESET)%s\n", $$1, $$2}'
