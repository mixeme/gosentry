#!/usr/bin/env bash
set -euo pipefail

# Build all release artifacts from a Linux host or CI runner. The Docker image
# contains Linux/Fyne dependencies for amd64 and arm64, plus the MinGW
# cross-compiler used for the Windows GUI executable.
tag="gitea.mixdep.ru/mix/pysentry-builder"

docker build -f Dockerfile -t "$tag" .

container_id="$(docker create "$tag")"
cleanup() {
    docker rm "$container_id" >/dev/null
}
trap cleanup EXIT

mkdir -p dist/linux dist/windows
docker cp "${container_id}:/out/linux/." dist/linux
docker cp "${container_id}:/out/windows/." dist/windows

echo "Built release artifacts:"
find dist/linux dist/windows -maxdepth 1 -type f -print
