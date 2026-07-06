APP=nickpit
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

# Embed only the tree-sitter grammars the retrieval backends use (python, rust;
# the JS/TS family is parsed by esbuild). Without these tags a plain `go build`
# still works but embeds all ~200 grammars (~24 MB larger binary).
GRAMMAR_TAGS = grammar_subset,grammar_subset_python,grammar_subset_rust

.DEFAULT_GOAL := build

.PHONY: help generate build debug install test race lint modernize vet fmt

.DEFAULT:
	@echo "Error: unknown target '$@'"
	@echo ""
	@$(MAKE) --no-print-directory help
	@exit 1

generate: ## Generate checked-in files
	go generate ./internal/config ./workflows

build: generate ## Build the nickpit binary into ./bin
	mkdir -p ./bin
	go build -tags "$(GRAMMAR_TAGS)" -o ./bin/$(APP) ./cmd/$(APP)

debug: generate ## Build debug version of nickpit binary into ./bin
	mkdir -p ./bin
	go build -tags "$(GRAMMAR_TAGS)" -o ./bin/$(APP) -gcflags "-N -l" ./cmd/$(APP)

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "%-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: build ## Install the binary to $(BINDIR)
	install -m 0755 ./bin/$(APP) $(BINDIR)/$(APP)

test: ## Run the test suite
	go test -tags "$(GRAMMAR_TAGS)" ./...

race: ## Run the race detector
	go test -tags "$(GRAMMAR_TAGS)" -race ./...

lint: ## Run golangci-lint (install: https://golangci-lint.run/welcome/install/)
	golangci-lint run ./...

modernize: ## Run gopls modernize analyzers (add -fix to auto-apply)
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@v0.22.0 ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format Go source files
	gofmt -w ./cmd ./internal
