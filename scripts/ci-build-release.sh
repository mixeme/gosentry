#!/usr/bin/env bash
set -euo pipefail

# Build and package every release artifact on a Linux host that already has the
# cross toolchain installed (native gcc + X11/OpenGL headers, the aarch64 cross
# compiler, and the MinGW-w64 toolchain for the Windows GUI binary). This is the
# non-Docker counterpart to scripts/build-release-linux.sh: the CI workflows in
# .github/ and .forgejo/ install those packages directly on the runner and then
# call this script, so the exact build/package commands live in one place and do
# not drift between the two forges.
#
# The build flags mirror the other scripts intentionally: -trimpath strips local
# paths, -s -w drops symbol/debug tables to shrink the binaries, -H=windowsgui
# suppresses the console window on Windows, and -X injects the version so the
# GUI and artifact names agree.

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "$repo_root"

# VERSION can be provided by CI (for a tagged release the workflow passes the tag
# without its leading "v"). Fall back to the source of truth in version.go so the
# script also works for a plain local invocation.
version="${VERSION:-$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/app/version.go | tr -d '\r')}"
version="${version:-0.0.0-dev}"
ldflags="-s -w -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${version}"

echo "Building GoSentry ${version} release artifacts"
mkdir -p dist/linux dist/windows

# --- Linux amd64 -----------------------------------------------------------
echo "==> linux/amd64"
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
    go build -buildvcs=false -trimpath -ldflags "$ldflags" \
    -o "dist/linux/gosentry-${version}-linux-amd64" ./cmd/gosentry

# --- Linux arm64 (cross compiled) ------------------------------------------
echo "==> linux/arm64"
CC=aarch64-linux-gnu-gcc \
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
CGO_CFLAGS="--sysroot=/ -I/usr/include/aarch64-linux-gnu" \
CGO_LDFLAGS="--sysroot=/ -L/usr/lib/aarch64-linux-gnu" \
PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig \
    go build -buildvcs=false -trimpath -ldflags "$ldflags" \
    -o "dist/linux/gosentry-${version}-linux-arm64" ./cmd/gosentry

# --- Windows amd64 (cross compiled with MinGW) -----------------------------
echo "==> windows/amd64"
# windres embeds the .ico into the PE resource so Explorer/taskbar show the icon.
# The .syso is suffixed windows_amd64, so Go only links it into the Windows build
# and ignores it for the Linux targets above.
x86_64-w64-mingw32-windres -O coff \
    -o cmd/gosentry/rsrc_windows_amd64.syso packaging/windows/gosentry.rc
CC=x86_64-w64-mingw32-gcc \
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
    go build -buildvcs=false -trimpath -ldflags "-H=windowsgui ${ldflags}" \
    -o "dist/windows/gosentry-${version}-windows-amd64.exe" ./cmd/gosentry

# --- Package ---------------------------------------------------------------
# Each archive holds the executable plus the top-level README and CHANGELOG,
# flattened to the archive root so a user can extract straight into any folder.
# This matches the layout produced by package-linux.sh / package-windows.bat.
package_linux() {
    local arch="$1"
    local binary="dist/linux/gosentry-${version}-linux-${arch}"
    local tarball="dist/linux/gosentry-${version}-linux-${arch}.tar.gz"
    local staging="dist/linux/_staging-${arch}"

    rm -rf "$staging"
    mkdir -p "$staging"
    cp "$binary" "$staging/gosentry"
    cp README.md "$staging/README.md"
    cp docs/CHANGELOG.md "$staging/CHANGELOG.md"
    tar -czf "$tarball" -C "$staging" .
    rm -rf "$staging"
    echo "Packaged $tarball"
}

package_windows() {
    local binary="dist/windows/gosentry-${version}-windows-amd64.exe"
    local zipfile="gosentry-${version}-windows-amd64.zip"
    local staging="dist/windows/_staging-amd64"

    rm -rf "$staging"
    mkdir -p "$staging"
    cp "$binary" "$staging/gosentry.exe"
    cp README.md "$staging/README.md"
    cp docs/CHANGELOG.md "$staging/CHANGELOG.md"
    # -j flattens: files land at the zip root with no staging path prefix.
    ( cd "$staging" && zip -j -q "../${zipfile}" ./* )
    rm -rf "$staging"
    echo "Packaged dist/windows/${zipfile}"
}

package_linux amd64
package_linux arm64
package_windows

echo "Release artifacts:"
find dist/linux dist/windows -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) -print
