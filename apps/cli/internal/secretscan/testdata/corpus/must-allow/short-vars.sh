#!/bin/bash
# Build script — no credentials

set -euo pipefail

export BUILD_TARGET="linux/amd64"
export VERSION="v2.0.0"
export OUTPUT_DIR="./dist"

# These are short — well below the 20-char threshold for generic detection
SECRET_NAME="not-long-enough"
TOKEN_TYPE="Bearer"
KEY_NAME="public-key-id-only"

go build -o "${OUTPUT_DIR}/vedox-${VERSION}" ./cmd/vedox/...
