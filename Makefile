APP=nickpit

.PHONY: build test lint fmt

build:
	mkdir -p ./bin
	go build -o ./bin/$(APP) ./cmd/$(APP)

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal
