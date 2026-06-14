#!/usr/bin/env bash
set -euo pipefail

output="${1:-dist/linux/pysentry}"
mkdir -p "$(dirname "$output")"

export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

go build -trimpath -ldflags "-s -w" -o "$output" ./cmd/pysentry
rm -rf "$(dirname "$output")/assets"
cp -R assets "$(dirname "$output")/assets"

echo "Built $output"
