#!/bin/bash

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
GO_DIR="$ROOT_DIR/src"

if [ ! -f "$GO_DIR/go.mod" ]; then
    echo "go.mod not found in $GO_DIR."
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Go not found. Please install Go and ensure it's in your PATH."
    exit 1
fi

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"

# Architecture-specific artifact naming:
#   llvm-configure-linux-amd64, llvm-configure-darwin-arm64, ...
BIN_NAME="llvm-configure-${GOOS}-${GOARCH}"

(cd "$GO_DIR" && CGO_ENABLED=0 go build -ldflags="-s -w" -o "$ROOT_DIR/build/$BIN_NAME" ./cmd/llvm-configure)

if [ $? -ne 0 ]; then
    echo "Build failed."
    exit 1
fi

# UPX compression:
#   - the binary is written to build/, not the repository root
#   - UPX cannot compress Mach-O arm64 artifacts, so only attempt it on
#     Linux/amd64, where it is known to work
if [ "$GOOS" = "linux" ] && [ "$GOARCH" = "amd64" ] && command -v upx &> /dev/null
then
    echo "UPX found, will compress the binary."
    upx --best "$ROOT_DIR/build/$BIN_NAME"
else
    echo "UPX compression skipped (supported for linux-amd64 artifacts only)."
fi