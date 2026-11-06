BIN     := bin/family-hub
# Keep in sync with the golangci-lint version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := 2.15.0

DATABASE_URL      ?= postgres://app:app@localhost:5432/familyhub?sslmode=disable
# Integration tests use only this, so they never touch the dev database.
TEST_DATABASE_URL ?= postgres://app:app@localhost:5432/familyhub_test?sslmode=disable
export DATABASE_URL TEST_DATABASE_URL

.PHONY: up down build run test lint vuln migrate-up migrate-down

up:
	docker compose up -d --wait
	@printf '%s\n' "SELECT 'CREATE DATABASE familyhub_test' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'familyhub_test')\gexec" \
	  | docker compose exec -T postgres sh -c 'psql -q -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

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

migrate-up:
	go run ./cmd migrate up

# e.g. `make migrate-down N=2`
migrate-down:
	go run ./cmd migrate down $(or $(N),1)
