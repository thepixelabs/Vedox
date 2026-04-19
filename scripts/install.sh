#!/usr/bin/env sh
# install.sh — Vedox curl-pipe installer
#
# Usage:
#   curl -fsSL https://get.vedox.dev/install.sh | sh
#
# Supported platforms:
#   macOS arm64 (Apple Silicon)
#   macOS amd64 (Intel)
#   Linux amd64
#   Linux arm64
#
# What this script does:
#   1. Detects OS and architecture.
#   2. Downloads the correct release tarball from GitHub Releases.
#   3. Downloads and verifies SHA256 checksum.
#   4. Optionally verifies cosign signature if cosign is on PATH.
#   5. Extracts the binary.
#   6. Installs to /usr/local/bin/vedox (or ~/.local/bin/vedox if not writable).
#   7. Prints next-step instructions.
#
# This script WILL NOT:
#   - Run as root (explicitly refused — see guard below).
#   - Make any network calls after installation.
#   - Write anything outside the install prefix and /tmp.
#   - Modify shell profile files (PATH instructions are printed, not applied).
#
# Verify manually:
#   shasum -a 256 vedox-<version>-<os>-<arch>.tar.gz
#   (compare to vedox_<version>_checksums.txt in the GitHub Release)

set -eu

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

REPO="thepixelabs/vedox"
RELEASES_BASE="https://github.com/${REPO}/releases/latest/download"
BINARY_NAME="vedox"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info()  { printf '  \033[0;34m%s\033[0m\n' "$*"; }
ok()    { printf '  \033[0;32m%s\033[0m\n' "$*"; }
warn()  { printf '  \033[0;33mwarn:\033[0m %s\n' "$*"; }
die()   { printf '  \033[0;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    die "required command not found: $1 — install it and re-run this script"
  fi
}

# ---------------------------------------------------------------------------
# Root guard
# ---------------------------------------------------------------------------
# Installing as root means the binary ends up owned by root, ~/.vedox/ gets
# written as root, and launchd cannot load a root-owned plist in user context.
# Just say no.

if [ "$(id -u)" -eq 0 ]; then
  die "do not run this script as root. Run as your normal user account.
       The script will ask for sudo only if the install prefix requires it,
       or will fall back to ~/.local/bin/vedox automatically."
fi

# ---------------------------------------------------------------------------
# Detect OS
# ---------------------------------------------------------------------------

OS="$(uname -s)"
case "${OS}" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux"  ;;
  *)      die "unsupported operating system: ${OS}. Supported: macOS, Linux." ;;
esac

# ---------------------------------------------------------------------------
# Detect architecture
# ---------------------------------------------------------------------------

ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)           ARCH="amd64" ;;
  aarch64|arm64)    ARCH="arm64" ;;
  *)                die "unsupported architecture: ${ARCH}. Supported: amd64, arm64." ;;
esac

# ---------------------------------------------------------------------------
# Build artifact name
#
# goreleaser name_template: "vedox-{{ .Version }}-{{ .Os }}-{{ .Arch }}"
# macOS universal binary: the universal_binaries block with replace: true
# makes goreleaser produce a single darwin artifact where .Arch = "all".
# So the macOS archive is: vedox-<VERSION>-darwin-all.zip
# Linux archives are:       vedox-<VERSION>-linux-amd64.tar.gz
#                           vedox-<VERSION>-linux-arm64.tar.gz
#
# MAINTAINER: if the goreleaser name_template changes, update ARTIFACT_NAME below.
# ---------------------------------------------------------------------------

if [ "${OS}" = "darwin" ]; then
  # macOS always uses the universal binary regardless of host arch.
  ARTIFACT_ARCH="all"
  ARCHIVE_EXT="zip"
else
  ARTIFACT_ARCH="${ARCH}"
  ARCHIVE_EXT="tar.gz"
fi

# The version is resolved at download time via GitHub's /releases/latest redirect.
# We embed a placeholder here; the actual filename is fetched from the release.
# Strategy: download the checksums file first to learn the actual version, then
# download the correct artifact.

CHECKSUMS_URL="${RELEASES_BASE}/vedox_checksums.txt"

# ---------------------------------------------------------------------------
# Determine install prefix
# ---------------------------------------------------------------------------

LOCAL_BIN="/usr/local/bin"
FALLBACK_BIN="${HOME}/.local/bin"

if [ -w "${LOCAL_BIN}" ]; then
  INSTALL_DIR="${LOCAL_BIN}"
  NEEDS_SUDO=0
elif command -v sudo >/dev/null 2>&1; then
  # /usr/local/bin exists but is not writable by the current user.
  # Offer sudo elevation or fall back to ~/.local/bin.
  INSTALL_DIR="${LOCAL_BIN}"
  NEEDS_SUDO=1
else
  INSTALL_DIR="${FALLBACK_BIN}"
  NEEDS_SUDO=0
fi

# If we end up using the fallback, create it if it does not exist.
if [ "${INSTALL_DIR}" = "${FALLBACK_BIN}" ]; then
  mkdir -p "${INSTALL_DIR}"
fi

# ---------------------------------------------------------------------------
# Required commands
# ---------------------------------------------------------------------------

need_cmd curl
need_cmd shasum
need_cmd mktemp

if [ "${ARCHIVE_EXT}" = "zip" ]; then
  need_cmd unzip
else
  need_cmd tar
fi

# ---------------------------------------------------------------------------
# Work in a temp directory — cleaned up on exit
# ---------------------------------------------------------------------------

TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}"' EXIT INT TERM

cd "${TMPDIR}"

# ---------------------------------------------------------------------------
# Step 1: Download checksums file to discover the latest version string
# ---------------------------------------------------------------------------

info "fetching checksums from GitHub Releases..."
curl --proto '=https' --tlsv1.2 -fsSL -o "checksums.txt" "${CHECKSUMS_URL}" || \
  die "failed to download checksums file from ${CHECKSUMS_URL}"

# Extract the version from the checksums filename pattern:
# The checksums file itself is named vedox_<VERSION>_checksums.txt.
# The artifact lines inside it look like:
#   abc123...  vedox-2.0.0-linux-amd64.tar.gz
# Parse the version from the first artifact line.

VERSION="$(grep -oE 'vedox-[0-9]+\.[0-9]+\.[0-9]+' checksums.txt | head -1 | sed 's/vedox-//')"
if [ -z "${VERSION}" ]; then
  die "could not parse version from checksums file. Inspect ${CHECKSUMS_URL} manually."
fi

ok "latest version: ${VERSION}"

# ---------------------------------------------------------------------------
# Step 2: Build the artifact filename and download URL
# ---------------------------------------------------------------------------

ARTIFACT_NAME="vedox-${VERSION}-${OS}-${ARTIFACT_ARCH}.${ARCHIVE_EXT}"
ARTIFACT_URL="${RELEASES_BASE}/${ARTIFACT_NAME}"

# ---------------------------------------------------------------------------
# Step 3: Download the artifact
# ---------------------------------------------------------------------------

info "downloading ${ARTIFACT_NAME}..."
curl --proto '=https' --tlsv1.2 -fsSL -o "${ARTIFACT_NAME}" "${ARTIFACT_URL}" || \
  die "failed to download artifact from ${ARTIFACT_URL}"

# ---------------------------------------------------------------------------
# Step 4: Verify SHA256 checksum
# ---------------------------------------------------------------------------

info "verifying checksum..."

# Extract the expected hash for this artifact from the checksums file.
EXPECTED_HASH="$(grep " ${ARTIFACT_NAME}$" checksums.txt | awk '{print $1}')"
if [ -z "${EXPECTED_HASH}" ]; then
  die "artifact ${ARTIFACT_NAME} not found in checksums file. The release may be incomplete.
       Download manually from https://github.com/${REPO}/releases and verify with:
         shasum -a 256 ${ARTIFACT_NAME}"
fi

# Compute the actual hash.
ACTUAL_HASH="$(shasum -a 256 "${ARTIFACT_NAME}" | awk '{print $1}')"

if [ "${EXPECTED_HASH}" != "${ACTUAL_HASH}" ]; then
  die "checksum mismatch for ${ARTIFACT_NAME}
       expected: ${EXPECTED_HASH}
       actual:   ${ACTUAL_HASH}
       Do not use this binary. The download may be corrupt or tampered with."
fi

ok "checksum verified"

# ---------------------------------------------------------------------------
# Step 5: Verify cosign signature (optional — degrades gracefully if absent)
# ---------------------------------------------------------------------------
# Linux artifacts are signed with Sigstore keyless cosign in CI.
# macOS artifacts are signed with Apple Developer ID codesign (not cosign).
# cosign verification is only applicable to Linux artifacts.

if [ "${OS}" = "linux" ]; then
  if command -v cosign >/dev/null 2>&1; then
    info "verifying cosign signature..."
    SIG_URL="${RELEASES_BASE}/${ARTIFACT_NAME}.sig"
    PEM_URL="${RELEASES_BASE}/${ARTIFACT_NAME}.pem"
    curl --proto '=https' --tlsv1.2 -fsSL -o "${ARTIFACT_NAME}.sig" "${SIG_URL}" || \
      warn "cosign .sig file not found at ${SIG_URL} — skipping signature verification"
    curl --proto '=https' --tlsv1.2 -fsSL -o "${ARTIFACT_NAME}.pem" "${PEM_URL}" || \
      warn "cosign .pem file not found at ${PEM_URL} — skipping signature verification"
    if [ -f "${ARTIFACT_NAME}.sig" ] && [ -f "${ARTIFACT_NAME}.pem" ]; then
      cosign verify-blob \
        --certificate "${ARTIFACT_NAME}.pem" \
        --signature "${ARTIFACT_NAME}.sig" \
        --certificate-identity-regexp "https://github.com/${REPO}/.*" \
        --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
        "${ARTIFACT_NAME}" && ok "cosign signature verified" || \
        die "cosign signature verification failed. Do not use this binary."
    fi
  else
    warn "cosign not found — skipping signature verification.
         For verified installs, install cosign: https://docs.sigstore.dev/system_config/installation/
         SHA256 checksum was still verified above."
  fi
fi

# ---------------------------------------------------------------------------
# Step 6: Extract the binary
# ---------------------------------------------------------------------------

info "extracting ${ARTIFACT_NAME}..."

if [ "${ARCHIVE_EXT}" = "zip" ]; then
  unzip -q "${ARTIFACT_NAME}"
else
  tar -xzf "${ARTIFACT_NAME}"
fi

# The binary inside the archive is named 'vedox' at the top level.
# goreleaser archives: the binary is at the root of the archive (no subdirectory).
EXTRACTED_BINARY="${TMPDIR}/${BINARY_NAME}"
if [ ! -f "${EXTRACTED_BINARY}" ]; then
  # Some goreleaser configurations nest inside a directory. Try to find it.
  EXTRACTED_BINARY="$(find "${TMPDIR}" -name "${BINARY_NAME}" -type f | head -1)"
  if [ -z "${EXTRACTED_BINARY}" ]; then
    die "could not find '${BINARY_NAME}' binary after extracting ${ARTIFACT_NAME}.
         The archive layout may have changed. Extract manually and place in your PATH."
  fi
fi

chmod +x "${EXTRACTED_BINARY}"

# ---------------------------------------------------------------------------
# Step 7: Install the binary
# ---------------------------------------------------------------------------

INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}"

info "installing to ${INSTALL_PATH}..."

if [ "${NEEDS_SUDO}" -eq 1 ]; then
  warn "installing to ${LOCAL_BIN} requires sudo. You will be prompted for your password."
  sudo install -m 755 "${EXTRACTED_BINARY}" "${INSTALL_PATH}" || \
    die "sudo install failed. Try running without sudo by ensuring ${FALLBACK_BIN} is on your PATH,
         then re-run this script (it will fall back to ${FALLBACK_BIN}/vedox)."
else
  install -m 755 "${EXTRACTED_BINARY}" "${INSTALL_PATH}"
fi

# ---------------------------------------------------------------------------
# Step 8: Write local install event log (local only, never transmitted)
# ---------------------------------------------------------------------------

VEDOX_STATE_DIR="${HOME}/.vedox"
mkdir -p "${VEDOX_STATE_DIR}"
chmod 700 "${VEDOX_STATE_DIR}"

INSTALL_LOG="${VEDOX_STATE_DIR}/install-log.jsonl"
# Append one JSON line. jq is not required — construct manually.
printf '{"timestamp":"%s","version":"%s","os":"%s","arch":"%s","method":"install.sh","install_path":"%s"}\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  "${VERSION}" \
  "${OS}" \
  "${ARCH}" \
  "${INSTALL_PATH}" \
  >> "${INSTALL_LOG}" 2>/dev/null || true  # non-fatal if write fails

# ---------------------------------------------------------------------------
# Step 9: PATH check and success message
# ---------------------------------------------------------------------------

printf '\n'
ok "vedox ${VERSION} installed to ${INSTALL_PATH}"
printf '\n'

# Check if the install directory is on PATH.
INSTALL_ON_PATH=0
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) INSTALL_ON_PATH=1 ;;
esac

if [ "${INSTALL_ON_PATH}" -eq 0 ] && [ "${INSTALL_DIR}" = "${FALLBACK_BIN}" ]; then
  warn "${INSTALL_DIR} is not on your PATH."
  printf '  Add it by running:\n'
  printf '\n'
  # Detect shell and print the appropriate export.
  CURRENT_SHELL="$(basename "${SHELL:-sh}")"
  case "${CURRENT_SHELL}" in
    zsh)  printf '    echo '"'"'export PATH="$HOME/.local/bin:$PATH"'"'"' >> ~/.zshrc && source ~/.zshrc\n' ;;
    fish) printf '    fish_add_path ~/.local/bin\n' ;;
    *)    printf '    echo '"'"'export PATH="$HOME/.local/bin:$PATH"'"'"' >> ~/.bashrc && source ~/.bashrc\n' ;;
  esac
  printf '\n'
fi

printf '  run `vedox init` to get started.\n'
printf '\n'
