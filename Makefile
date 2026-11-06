BIN     := bin/family-hub
VERSION ?= dev

.PHONY: build run test vet

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/family-hub

# Pass subcommand and flags via ARGS, e.g. `make run ARGS="version"`.
run:
	go run ./cmd/family-hub $(ARGS)

test:
	go test ./...

vet:
	go vet ./...
