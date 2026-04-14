APP=nickpit

.PHONY: build test lint fmt

build:
	go build ./...

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal
