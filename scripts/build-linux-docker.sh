#!/usr/bin/env bash
set -euo pipefail

# Optional first argument mirrors build-linux.sh. The Docker build still writes
# the final artifact into the local dist/ tree, not into the container. The
# default includes the application version and target platform.
version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/core/version.go)"
version="${version:-0.0.0-dev}"
output="${1:-dist/linux/pysentry-${version}-linux-amd64}"

# Dockerfile contains the native packages required by Fyne. Keeping that
# environment in Docker makes Linux builds repeatable from Windows hosts and CI.
docker build -f Dockerfile -t gitea.mixdep.ru/mix/pysentry-builder .

# The image build produces /out/linux and /out/windows. This helper copies only
# the Linux binary for compatibility with the older Linux-only workflow; use
# build-release-linux.sh when both platform artifacts are needed.
container_id="$(docker create gitea.mixdep.ru/mix/pysentry-builder)"
cleanup() {
    docker rm "$container_id" >/dev/null
}
trap cleanup EXIT

mkdir -p "$(dirname "$output")"
docker cp "${container_id}:/out/linux/pysentry-${version}-linux-amd64" "$output"

# Icons are embedded in the Go binary, so there is no assets directory to copy
# after extracting the Linux executable.
echo "Built $output"
