# Base images are pinned to exact tags; Dependabot proposes updates.
# The Go version must be at least the `go` line in go.mod.
FROM golang:1.27.3-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
RUN apk add --no-cache make
COPY . .
# The Makefile's build, so the image and CI build the same way (with the placeholder UI until Phase 4a).
RUN make build

# Phase 6 adds ffmpeg and vips here.
FROM alpine:3.24.4
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=build /src/bin/family-hub .
EXPOSE 8080
# The port from HTTP_ADDR (default :8080); the app must listen on all interfaces or loopback.
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s \
  CMD addr="${HTTP_ADDR:-:8080}"; wget -qO- "http://127.0.0.1:${addr##*:}/health" || exit 1
ENTRYPOINT ["./family-hub"]
