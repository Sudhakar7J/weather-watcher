# syntax=docker/dockerfile:1

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
# Pinned to the Go version declared in go.mod so the image and local builds
# agree. Alpine keeps this stage small; it is discarded and never ships.
FROM golang:1.23-alpine AS build

WORKDIR /src

# Copy manifests first so dependency resolution is cached independently of
# source changes. There are no third-party dependencies today, so this is
# future-proofing rather than a current speed-up.
COPY go.mod ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary, which is what lets the runtime stage
# use a distroless base with no libc at all.
# -trimpath strips filesystem paths, and -ldflags="-s -w" drops the symbol
# table and DWARF data, making the binary smaller and harder to fingerprint.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/server \
        ./cmd/server

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
# Distroless contains no shell and no package manager, so the runtime image has
# no attack surface from either. The `nonroot` variant ships a uid that already
# exists, so no useradd step is needed.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="Weather Watcher API" \
      org.opencontainers.image.description="Go service exposing cached Open-Meteo weather data" \
      org.opencontainers.image.source="https://github.com/incubrix/weather-watcher"

COPY --from=build /out/server /server

# The service reads PORT from the environment and defaults to 8080.
# AWS App Runner sets this automatically.
EXPOSE 8080

USER nonroot:nonroot

# Exec form: the binary receives SIGTERM directly, which is what the graceful
# shutdown path in cmd/server relies on. A shell-form ENTRYPOINT would spawn a
# shell as PID 1 and swallow the signal.
ENTRYPOINT ["/server"]
