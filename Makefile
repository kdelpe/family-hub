BIN     := bin/family-hub
VERSION ?= $(or $(shell git describe --tags --always --dirty 2>/dev/null),dev)

.PHONY: build run test vet

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd

# Pass subcommand and flags via ARGS, e.g. `make run ARGS="version"`.
run:
	go run ./cmd $(ARGS)

test:
	go test ./...

vet:
	go vet ./...
