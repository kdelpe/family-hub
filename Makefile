BIN     := bin/family-hub

.PHONY: build run test vet

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd

# Pass subcommand and flags via ARGS, e.g. `make run ARGS="migrate up"`.
run:
	go run ./cmd $(ARGS)

test:
	go test ./...

vet:
	go vet ./...
