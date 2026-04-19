# syntax=docker/dockerfile:1.7
# Vedox — runtime-only container.
#
# This Dockerfile is built by goreleaser's `dockers:` block. The build context
# is `dist/vedox-{linux-amd64,linux-arm64}/` — a directory containing the
# prebuilt, statically-linked Go binary. The SvelteKit editor is already
# embedded into the binary at goreleaser build time (see before.hooks +
# //go:build release in apps/cli/internal/webassets/embed.go), so this
# Dockerfile only needs to ship the runtime layer.
#
# DO NOT add multi-stage build steps here that try to compile from source —
# goreleaser does not stage the source tree into the docker context. If you
# need a standalone `docker build .` from the repo root, run goreleaser locally
# first or use a separate Dockerfile.dev.
#
# Image: gcr.io/distroless/static-debian12:nonroot
#   - no shell, no libc, no package manager (~2 MB base)
#   - default UID 65532 (nonroot)
#   - CGO_ENABLED=0 binary, no glibc dependency
#
# Deploy mode auto-detection:
#   The daemon detects /.dockerenv at startup and switches to container mode.
#   Override with VEDOX_DEPLOY_MODE=container if needed.

FROM gcr.io/distroless/static-debian12:nonroot

# OCI image labels — version/revision/created are injected by goreleaser via
# --label build_flag_templates in .goreleaser.yaml. These static labels are
# always present.
LABEL org.opencontainers.image.title="vedox" \
      org.opencontainers.image.description="Local-first, Git-native documentation operating system for solo developers" \
      org.opencontainers.image.url="https://vedox.dev" \
      org.opencontainers.image.source="https://github.com/thepixelabs/Vedox" \
      org.opencontainers.image.vendor="Pixelabs" \
      org.opencontainers.image.licenses="PolyForm Shield 1.0.0"

# Goreleaser stages the binary into the build context as `vedox` (the project
# name from the builds.binary template).
COPY vedox /vedox

# /home/nonroot/.vedox is the daemon's state directory (registry, global.db,
# user-prefs.json, secrets.age, logs, crashes). MUST be volume-mounted for
# state persistence across container restarts.
VOLUME ["/home/nonroot/.vedox"]

# Default daemon port (override with VEDOX_PORT). EXPOSE is documentation
# only — operators must publish explicitly and bind to 127.0.0.1 in compose.
EXPOSE 5150/tcp

# Healthcheck: distroless has no curl/wget/sh — the binary itself is the
# only valid probe. --start-period gives the daemon time for migrations.
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/vedox", "server", "status", "--json"]

# Run as PID 1 (no shell wrapper). --deploy-mode=container disables
# launchd/systemd paths and relocates the PID file under /run/vedox/.
ENTRYPOINT ["/vedox", "server", "start", "--foreground", "--deploy-mode=container"]
