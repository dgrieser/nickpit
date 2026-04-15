APP=nickpit
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

.PHONY: help build install test lint fmt

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9_.-]+:.*## / {printf "%-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the nickpit binary into ./bin
	mkdir -p ./bin
	go build -o ./bin/$(APP) ./cmd/$(APP)

install: build ## Install the binary to $(BINDIR)
	install -m 0755 ./bin/$(APP) $(BINDIR)/$(APP)

test: ## Run the test suite
	go test ./...

lint: ## Run go vet
	go vet ./...

fmt: ## Format Go source files
	gofmt -w ./cmd ./internal
