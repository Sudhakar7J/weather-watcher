# syntax=docker/dockerfile:1

# Two entrypoints from one codebase. The default builds the long-running HTTP
# server; `--target lambda` builds the AWS Lambda variant, which is the same
# code behind a different adapter (cmd/lambda) rather than a fork.
#
# The bases differ on purpose: Lambda invokes the image through the runtime
# interface client and needs the entrypoint wrapper that AWS-provided base
# images carry. Distroless has no shell and therefore no wrapper, so an
# identical build there fails every invoke with Runtime.InvalidEntrypoint.

# ---------------------------------------------------------------------------
# Build stage (shared)
# ---------------------------------------------------------------------------
# Go version tracks go.mod so the image and local builds agree.
FROM golang:1.26-alpine AS build

ARG TARGET
WORKDIR /src

# Manifests first so dependency resolution is cached independently of source
# changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 produces a static binary. -trimpath strips filesystem paths and
# -ldflags="-s -w" drops the symbol table and DWARF data.
RUN CGO_ENABLED=0 GOOS=linux go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /out/app \
        ./cmd/${TARGET}

# ---------------------------------------------------------------------------
# Runtime: long-running server
# ---------------------------------------------------------------------------
# Distroless carries no shell and no package manager, so the image has no
# attack surface from either. The `nonroot` variant already has a uid.
FROM gcr.io/distroless/static-debian12:nonroot AS server

LABEL org.opencontainers.image.title="Weather Watcher API" \
      org.opencontainers.image.description="Go service exposing cached Open-Meteo weather data" \
      org.opencontainers.image.source="https://github.com/incubrix/weather-watcher"

COPY --from=build /out/app /app

# Exec form so the binary is PID 1 and receives SIGTERM directly, which is
# what the graceful shutdown path in cmd/server relies on.
ENTRYPOINT ["/app"]

# ---------------------------------------------------------------------------
# Runtime: AWS Lambda
# ---------------------------------------------------------------------------
FROM public.ecr.aws/lambda/provided:al2023 AS lambda

LABEL org.opencontainers.image.title="Weather Watcher API (Lambda)" \
      org.opencontainers.image.description="Go service exposing cached Open-Meteo weather data"

# The binary must be named `bootstrap`. The provided.al2023 entrypoint looks
# for /var/task/bootstrap and fails every invoke with "no bootstrap found"
# otherwise. The Go runtime interface client is linked into this binary, so
# `bootstrap` is the process Lambda starts.
COPY --from=build /out/app ${LAMBDA_TASK_ROOT}/bootstrap

# Not ENTRYPOINT: the base image's wrapper sets the runtime API variables and
# then execs this. Overriding ENTRYPOINT is what produced
# Runtime.InvalidEntrypoint before.
CMD [ "bootstrap" ]