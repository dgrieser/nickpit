APP=nickpit
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

# Embed only the tree-sitter grammars the retrieval backends use (python, rust;
# the JS/TS family is parsed by esbuild). Without these tags a plain `go build`
# still works but embeds all ~200 grammars (~24 MB larger binary).
GRAMMAR_TAGS = grammar_subset,grammar_subset_python,grammar_subset_rust

# Build identity, reported by `nickpit --version` and stamped into every review.
# Go can embed the revision itself, but only from a normal clone: its VCS
# detection expects ".git" to be a directory, and a linked git worktree has it
# as a file — a worktree build would report a bare "dev". So stamp it here.
# Both are overridable (`make build VERSION=v0.1.0`); a source tree with no git
# yields an empty COMMIT and the binary just says "dev". "-dirty" matches what
# Go's own vcs.modified reports: any uncommitted change, untracked files
# included.
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short=7 HEAD 2>/dev/null)$(shell git status --porcelain 2>/dev/null | grep -q . && echo -dirty)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

.DEFAULT_GOAL := build

.PHONY: help generate build debug install test race lint vet fmt

.DEFAULT:
	@echo "Error: unknown target '$@'"
	@echo ""
	@$(MAKE) --no-print-directory help
	@exit 1

generate: ## Generate checked-in files
	go generate ./internal/config ./workflows

build: generate ## Build the nickpit binary into ./bin
	mkdir -p ./bin
	go build -tags "$(GRAMMAR_TAGS)" -ldflags "$(LDFLAGS)" -o ./bin/$(APP) ./cmd/$(APP)

debug: generate ## Build debug version of nickpit binary into ./bin
	mkdir -p ./bin
	go build -tags "$(GRAMMAR_TAGS)" -ldflags "$(LDFLAGS)" -o ./bin/$(APP) -gcflags "-N -l" ./cmd/$(APP)

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "%-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: build ## Install the binary to $(BINDIR)
	install -m 0755 ./bin/$(APP) $(BINDIR)/$(APP)

test: ## Run the test suite
	go test -tags "$(GRAMMAR_TAGS)" ./...

race: ## Run the race detector
	go test -tags "$(GRAMMAR_TAGS)" -race ./...

lint: ## Run golangci-lint, modernize analyzers included (install: https://golangci-lint.run/welcome/install/)
	# A stale cache can report 0 issues on code CI still rejects, so start clean.
	golangci-lint cache clean
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format Go source files
	gofmt -w ./cmd ./internal
