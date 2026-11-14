BIN     := bin/family-hub
# Keep in sync with the golangci-lint version in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := 2.15.0
# Keep in sync with the mockery version and checksum in .github/workflows/ci.yml.
MOCKERY_VERSION := 3.8.1

DATABASE_URL      ?= postgres://app:app@localhost:5432/familyhub?sslmode=disable
TEST_DATABASE_URL ?= postgres://app:app@localhost:5432/familyhub_test?sslmode=disable
TEST_MAILPIT_URL  ?= http://localhost:8025
SMTP_HOST         ?= localhost
SMTP_PORT         ?= 1025
BASE_URL          ?= http://localhost:5173
export DATABASE_URL TEST_DATABASE_URL TEST_MAILPIT_URL SMTP_HOST SMTP_PORT BASE_URL

.PHONY: up down ui-stub dev-api generate build run test lint vuln migrate-up

up:
	docker compose up -d --wait
	@printf '%s\n' "SELECT 'CREATE DATABASE familyhub_test' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'familyhub_test')\gexec" \
	  | docker compose exec -T postgres sh -c 'psql -q -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

down:
	docker compose down

# A placeholder, so Go builds before the UI has been; never overwrites a real build.
ui-stub:
	@test -f internal/webui/dist/index.html || \
	  (mkdir -p internal/webui/dist && printf '%s\n' '<!doctype html><title>Family Hub</title><p>The UI is not built yet.</p>' > internal/webui/dist/index.html)

dev-api: up ui-stub
	AUTO_MIGRATE=true COOKIE_SECURE=false go run ./cmd

generate:
	@v=$$(mockery version 2>/dev/null); \
	  [ "$$v" = "v$(MOCKERY_VERSION)" ] || { echo "need mockery $(MOCKERY_VERSION), found '$$v'"; exit 1; }
	go tool sqlc generate
	mockery

build: ui-stub
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd

# Pass subcommand and flags via ARGS, e.g. `make run ARGS="migrate up"`.
run: ui-stub
	COOKIE_SECURE=false go run ./cmd $(ARGS)

# Unset so no test can reach the dev database; tests use TEST_DATABASE_URL.
test: ui-stub
	env -u DATABASE_URL go test -race ./...

lint: ui-stub
	@v=$$(golangci-lint version --short 2>/dev/null); \
	  [ "$$v" = "$(GOLANGCI_LINT_VERSION)" ] || { echo "need golangci-lint $(GOLANGCI_LINT_VERSION), found '$$v'"; exit 1; }
	golangci-lint run

vuln: ui-stub
	go tool govulncheck ./...

migrate-up: ui-stub
	go run ./cmd migrate up
