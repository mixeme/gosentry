#!/usr/bin/env bash
set -euo pipefail

# Move to the repository root regardless of where the script is invoked from.
cd "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/.."

version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/app/version.go | tr -d '\r')"
version="${version:-0.0.0-dev}"

package_arch() {
    local arch="$1"
    local binary="dist/linux/gosentry-${version}-linux-${arch}"
    local tarball="dist/linux/gosentry-${version}-linux-${arch}.tar.gz"
    local staging="dist/linux/_staging-${arch}"

    mkdir -p dist/linux

    if [ "$arch" = "arm64" ]; then
        if ! command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
            echo "Skipping linux/arm64: aarch64-linux-gnu-gcc not found."
            return 0
        fi
        echo "Building linux/arm64..."
        CC=aarch64-linux-gnu-gcc \
        CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
        CGO_CFLAGS="--sysroot=/ -I/usr/include/aarch64-linux-gnu" \
        CGO_LDFLAGS="--sysroot=/ -L/usr/lib/aarch64-linux-gnu" \
        PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig \
        go build -trimpath \
            -ldflags "-s -w -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${version}" \
            -o "$binary" ./cmd/gosentry
    else
        echo "Building linux/amd64..."
        CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
        go build -trimpath \
            -ldflags "-s -w -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${version}" \
            -o "$binary" ./cmd/gosentry
    fi

    rm -rf "$staging"
    mkdir -p "$staging"
    cp "$binary"         "$staging/gosentry"
    cp README.md         "$staging/README.md"
    cp docs/CHANGELOG.md "$staging/CHANGELOG.md"

    # -C "$staging" . puts all files at the archive root with no subdirectory.
    tar -czf "$tarball" -C "$staging" .
    rm -rf "$staging"

    echo "Packaged $tarball"
}

package_arch amd64
package_arch arm64
