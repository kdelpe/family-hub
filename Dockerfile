# Base images are pinned to exact tags; Dependabot proposes updates.
# The Go version must be at least the `go` line in go.mod.
FROM golang:1.27.4-alpine3.24 AS build
WORKDIR /src
RUN apk add --no-cache make
COPY . .
# The Makefile's build, so the image and CI build the same way (with the placeholder UI until Phase 4a).
# No `go mod download`: it would also fetch the sqlc tool's large dependency tree.
RUN --mount=type=cache,target=/go/pkg/mod make build

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
