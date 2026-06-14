#!/usr/bin/env bash
set -euo pipefail

# Optional first argument lets a developer or CI job choose the output path.
# dist/linux/pysentry is the default so generated binaries stay outside src/.
output="${1:-dist/linux/pysentry}"
mkdir -p "$(dirname "$output")"

# Fyne needs CGO for its native desktop backend. The script pins the target to
# linux/amd64 because this is the first supported Linux artifact; other
# architectures can be added later as explicit build targets.
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

# -trimpath removes local machine paths from debug/build metadata. -s -w strips
# symbol/debug tables to keep the desktop binary smaller.
go build -trimpath -ldflags "-s -w" -o "$output" ./cmd/pysentry

# The application icon is embedded by Go, so the Linux build does not need a
# sidecar assets directory beside the executable.
echo "Built $output"
