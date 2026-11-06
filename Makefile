BIN     := bin/family-hub
# Keep in sync with the golangci-lint version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := 2.15.0

.PHONY: up down build run test lint vuln

# --wait returns once Postgres passes its healthcheck.
up:
	docker compose up -d --wait

down:
	docker compose down

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd

# Pass subcommand and flags via ARGS, e.g. `make run ARGS="migrate up"`.
run:
	go run ./cmd $(ARGS)

test:
	go test -race ./...

lint:
	@v=$$(golangci-lint version --short 2>/dev/null); \
	  [ "$$v" = "$(GOLANGCI_LINT_VERSION)" ] || { echo "need golangci-lint $(GOLANGCI_LINT_VERSION), found '$$v'"; exit 1; }
	golangci-lint run

vuln:
	go tool govulncheck ./...
