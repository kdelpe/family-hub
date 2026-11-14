# Base images are pinned to exact tags; Dependabot proposes updates.
# Keep the Node version in sync with the web job in ci.yml.
FROM node:26.13.1-alpine3.24 AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
# Vite writes to ../internal/webui/dist, the folder the Go binary embeds.
RUN npm run build

# The Go version must be at least the `go` line in go.mod.
FROM golang:1.27.4-alpine3.24 AS build
WORKDIR /src
RUN apk add --no-cache make
COPY . .
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
# The Makefile's Go build, as CI runs it. No `go mod download`: it would also fetch the sqlc tool's large dependency tree.
RUN --mount=type=cache,target=/go/pkg/mod make build-go

# Phase 6 adds ffmpeg and vips here.
FROM alpine:3.24.4
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=build /src/bin/family-hub .
EXPOSE 8080
# The port from HTTP_ADDR, trimmed and defaulted as config.Load does; the app must listen on all interfaces or loopback.
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s \
  CMD addr=$(echo $HTTP_ADDR); addr="${addr:-:8080}"; wget -qO- "http://127.0.0.1:${addr##*:}/health" || exit 1
ENTRYPOINT ["./family-hub"]
