#!/usr/bin/env bash
set -euo pipefail

# Optional first argument mirrors build-linux.sh. The Docker build still writes
# the final artifact into the local dist/ tree, not into the container. The
# default includes the application version and target platform.
version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/core/version.go)"
version="${version:-0.0.0-dev}"
tag="gitea.mixdep.ru/mix/pysentry-builder:${version}"
output="${1:-dist/linux/pysentry-${version}-linux-amd64}"
docker_user_args=()
if command -v id >/dev/null 2>&1; then
    docker_user_args=(--user "$(id -u):$(id -g)")
fi

# Dockerfile contains the native packages required by Fyne. Keeping that
# environment in Docker makes Linux builds repeatable from Windows hosts and CI.
docker build -f Dockerfile -t "$tag" .

mkdir -p "$(dirname "$output")"
docker run --rm \
    "${docker_user_args[@]}" \
    -e "VERSION=${version}" \
    -e "OUTPUT=${output}" \
    -e "GOCACHE=/tmp/go-build-cache" \
    -v "$(pwd):/src" \
    -w /src \
    "$tag" \
    bash -c 'CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags "-s -w -X github.com/pysentry/pysentry/src/core.Version=${VERSION}" -o "${OUTPUT}" ./cmd/pysentry'

# Icons are embedded in the Go binary, so there is no assets directory to copy
# after extracting the Linux executable.
echo "Built $output"
