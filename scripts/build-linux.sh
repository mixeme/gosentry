#!/usr/bin/env bash
set -euo pipefail

# Optional first argument lets a developer or CI job choose the output path. The
# default includes the application version and target platform.
version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/app/version.go)"
version="${version:-0.0.0-dev}"
output="${1:-dist/linux/gosentry-${version}-linux-amd64}"
mkdir -p "$(dirname "$output")"

# Fyne needs CGO for its native desktop backend. The script pins the target to
# linux/amd64 because this is the first supported Linux artifact; other
# architectures can be added later as explicit build targets.
export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

# -trimpath removes local machine paths from debug/build metadata. -s -w strips
# symbol/debug tables to keep the desktop binary smaller.
go build -trimpath -ldflags "-s -w -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${version}" -o "$output" ./cmd/gosentry

# The application icon is embedded by Go, so the Linux build does not need a
# sidecar assets directory beside the executable.
echo "Built $output"
