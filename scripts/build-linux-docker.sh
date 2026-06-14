#!/usr/bin/env bash
set -euo pipefail

output="${1:-dist/linux/pysentry}"

docker build -f Dockerfile.linux -t pysentry-linux-builder .
container_id="$(docker create pysentry-linux-builder)"
mkdir -p "$(dirname "$output")"
docker cp "${container_id}:/out/pysentry" "$output"
docker rm "$container_id" >/dev/null

echo "Built $output"
