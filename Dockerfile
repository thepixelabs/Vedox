# syntax=docker/dockerfile:1.7
# Vedox — multi-stage container build
#
# Stages:
#   editor-build  — SvelteKit static export (node:22-bookworm-slim, build only)
#   go-build      — Go daemon compilation (golang:1.22-bookworm, build only)
#   runtime       — gcr.io/distroless/static-debian12:nonroot (~2 MB, shipped)
#
# Build:
#   docker build --target go-build .          # syntax check / builder test
#   docker build -t vedox:local .             # full image
#   docker build --platform linux/amd64,linux/arm64 -t vedox:local .   # multi-arch
#
# The goreleaser dockers: block builds this with --build-arg VERSION etc injected
# via build_flag_templates. Goreleaser also handles the SvelteKit pre-build via
# the before.hooks block; when building outside goreleaser, ensure you have already
# run `pnpm --filter @vedox/editor build` and the output is at apps/editor/build/.
#
# Deploy mode auto-detection (--deploy-mode=container):
#   The daemon detects /.dockerenv at startup and switches to container mode
#   automatically. You can override with VEDOX_DEPLOY_MODE=container explicitly.
#
# Security:
#   - Runs as UID 65532 (distroless nonroot) — never root
#   - No shell, no package manager in the runtime layer
#   - CGO_ENABLED=0: fully static binary
#   - Secrets via *_FILE env convention (Docker secrets mounted at /run/secrets/)

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# ── Stage 1: Build SvelteKit editor ──────────────────────────────────────────
FROM node:22-bookworm-slim AS editor-build
WORKDIR /workspace

# Copy package manifests first for maximum layer cache reuse.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/editor/package.json ./apps/editor/
COPY packages/ ./packages/

RUN corepack enable && \
    pnpm install --frozen-lockfile --filter @vedox/editor...

COPY apps/editor/ ./apps/editor/

RUN pnpm --filter @vedox/editor build
# Output lands at: apps/editor/build/

# ── Stage 2: Build Go daemon ─────────────────────────────────────────────────
# NOTE: apps/cli/go.mod requires go 1.25.0. golang:1.25-bookworm will be the
# correct pin once Go 1.25 ships (expected ~Aug 2025). Until then, golang:1.25rc1
# is the matching image. Update this tag when 1.25 final is released to Docker Hub.
# Do NOT downgrade to 1.22 or 1.24 — go mod download will fail with GOTOOLCHAIN=local.
FROM golang:1.25rc1-bookworm AS go-build
WORKDIR /src

# Copy go.mod and go.sum first so `go mod download` is a cached layer
# that only reruns when dependencies actually change.
COPY apps/cli/go.mod apps/cli/go.sum ./

# GOTOOLCHAIN=auto: go.mod requires 1.25.0; golang:1.25rc1 is pre-release and its
# version string (go1.25rc1) is treated as < 1.25.0 final by strict toolchain
# enforcement. GOTOOLCHAIN=auto allows the running toolchain to satisfy the
# requirement without attempting a network download in most CI environments.
# Once golang:1.25-bookworm (final) ships, switch image tag and remove this.
RUN GOTOOLCHAIN=auto go mod download

# Copy the full CLI source.
COPY apps/cli/ ./

# Copy the SvelteKit build output into the Go embed directory.
# The //go:build release guard in embed.go embeds from internal/webassets/editorassets/.
# When building via goreleaser the before.hooks block does this copy before
# invoking the docker build; in a standalone docker build we pull from stage 1.
# SvelteKit with adapter-auto outputs to .svelte-kit/output/client/ (static assets).
# When building via goreleaser, the before.hooks block pre-builds and copies to
# apps/editor/build/ then to editorassets/. In this standalone Docker build,
# we pull directly from the .svelte-kit output.
# NOTE: if your project switches to adapter-static, update this path to
# /workspace/apps/editor/build/ (matches goreleaser before.hooks convention).
COPY --from=editor-build /workspace/apps/editor/.svelte-kit/output/client/ ./internal/webassets/editorassets/

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=auto go build \
    -tags=release \
    -ldflags="-s -w \
      -X github.com/vedox/vedox/cmd.version=${VERSION} \
      -X github.com/vedox/vedox/cmd.commit=${COMMIT} \
      -X github.com/vedox/vedox/cmd.buildDate=${BUILD_DATE}" \
    -o /out/vedox .

# ── Stage 3: Runtime — distroless/static-debian12:nonroot ────────────────────
# Distroless static: no shell, no libc, no package manager.
# UID 65532 (nonroot) is the distroless default — no USER directive needed.
# Image size: ~2 MB base + ~25 MB binary = ~27 MB total compressed on GHCR.
FROM gcr.io/distroless/static-debian12:nonroot

# OCI image labels — goreleaser injects version/revision/created via
# --label build_flag_templates when building through the dockers: block.
# These static labels are always present.
LABEL org.opencontainers.image.title="vedox" \
      org.opencontainers.image.description="Local-first, Git-native documentation operating system for solo developers" \
      org.opencontainers.image.url="https://vedox.dev" \
      org.opencontainers.image.source="https://github.com/pixelicous/Vedox" \
      org.opencontainers.image.vendor="Pixelabs" \
      org.opencontainers.image.licenses="PolyForm Shield 1.0.0"

COPY --from=go-build /out/vedox /vedox

# /home/nonroot/.vedox is the daemon's state directory (registry, global.db,
# user-prefs.json, secrets.age, logs, crashes). MUST be volume-mounted for
# state persistence across container restarts.
VOLUME ["/home/nonroot/.vedox"]

# The daemon listens on port 5150 by default (portcheck.DefaultPort).
# Override with VEDOX_PORT env var. EXPOSE is documentation only — operators
# must publish explicitly and should bind to 127.0.0.1 in docker-compose.
EXPOSE 5150/tcp

# Healthcheck: vedox server status --json reads the PID file, calls GET /healthz
# internally on 127.0.0.1:<port>, and exits 0 on healthy, 69 on not-running.
# Distroless has no curl/wget/sh — the binary itself is the only valid probe.
# --start-period=30s gives the daemon time to open SQLite, run migrations,
# and load the repo registry before the first health check fires.
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/vedox", "server", "status", "--json"]

# ENTRYPOINT runs the binary directly (no shell wrapper — distroless has no sh).
# --foreground: run as PID 1 without detaching (container runtime IS the supervisor).
# --deploy-mode=container: skips launchd/systemd paths, relocates PID file to
#   /run/vedox/, disables Keychain probing, enables 0.0.0.0 bind (requires
#   VEDOX_BIND_ACK=i-understand-container-exposure to be set by the operator).
ENTRYPOINT ["/vedox", "server", "start", "--foreground", "--deploy-mode=container"]
