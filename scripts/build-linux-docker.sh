#!/usr/bin/env bash
set -euo pipefail

# Optional first argument mirrors build-linux.sh. The Docker build still writes
# the final artifact into the local dist/ tree, not into the container.
output="${1:-dist/linux/pysentry}"

# Dockerfile contains the native packages required by Fyne. Keeping that
# environment in Docker makes Linux builds repeatable from Windows hosts and CI.
docker build -f Dockerfile -t pysentry-linux-builder .

# The image build produces /out/pysentry. A temporary container is used only as a
# convenient way to copy that file out; the app is not run inside the container.
container_id="$(docker create pysentry-linux-builder)"
mkdir -p "$(dirname "$output")"
docker cp "${container_id}:/out/pysentry" "$output"
docker rm "$container_id" >/dev/null

# Icons are embedded in the Go binary, so there is no assets directory to copy
# after extracting the Linux executable.
echo "Built $output"
