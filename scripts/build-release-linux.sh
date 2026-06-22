#!/usr/bin/env bash
set -euo pipefail

# Build selected release artifacts from a Linux host or CI runner. The Docker
# image contains Linux/Fyne dependencies for amd64 and arm64, plus the MinGW
# cross-compiler used for the Windows GUI executable. Actual build commands live
# here rather than in Dockerfile so target selection does not require rebuilding
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "$repo_root"

version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/app/version.go | tr -d '\r')"
version="${version:-0.0.0-dev}"
tag="gitea.mixdep.ru/mix/gosentry-builder:${version}"

docker_user_args=()
if command -v id >/dev/null 2>&1; then
    docker_user_args=(--user "$(id -u):$(id -g)")
fi

usage() {
    cat <<EOF
Usage: $0 [target...]

Targets:
  all            Build every release artifact.
  linux-amd64    Build dist/linux/gosentry-${version}-linux-amd64.
  linux-arm64    Build dist/linux/gosentry-${version}-linux-arm64.
  windows-amd64  Build dist/windows/gosentry-${version}-windows-amd64.exe.

When no target is passed and the script runs in a terminal, it asks what to build.
EOF
}

for arg in "$@"; do
    case "$arg" in
        -h|--help|help)
            usage
            exit 0
            ;;
    esac
done

choose_targets() {
    if [ "$#" -gt 0 ]; then
        printf '%s\n' "$@"
        return
    fi

    if [ ! -t 0 ]; then
        printf '%s\n' all
        return
    fi

    echo "Select release artifacts to build:"
    echo "  1) all"
    echo "  2) linux-amd64"
    echo "  3) linux-arm64"
    echo "  4) windows-amd64"
    echo "Enter numbers or target names separated by spaces or commas."
    read -r -p "Build target [all]: " answer
    answer="${answer:-all}"
    echo "$answer" | tr ',' ' ' | tr ' ' '\n' | sed '/^$/d'
}

normalize_targets() {
    while IFS= read -r target; do
        case "$target" in
            1|all)
                printf '%s\n' linux-amd64 linux-arm64 windows-amd64
                ;;
            2|linux-amd64)
                printf '%s\n' linux-amd64
                ;;
            3|linux-arm64)
                printf '%s\n' linux-arm64
                ;;
            4|windows-amd64)
                printf '%s\n' windows-amd64
                ;;
            *)
                echo "Unknown build target: $target" >&2
                usage >&2
                exit 1
                ;;
        esac
    done
}

run_in_builder() {
    docker run --rm \
        "${docker_user_args[@]}" \
        -e "VERSION=${version}" \
        -e "GOCACHE=/tmp/go-build-cache" \
        -v "${repo_root}:/src" \
        -w /src \
        "$tag" \
        bash -c "$1"
}

build_linux_amd64() {
    run_in_builder 'mkdir -p dist/linux && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags "-s -w -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${VERSION}" -o "dist/linux/gosentry-${VERSION}-linux-amd64" ./cmd/gosentry'
}

build_linux_arm64() {
    run_in_builder 'mkdir -p dist/linux && CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CGO_CFLAGS="--sysroot=/ -I/usr/include/aarch64-linux-gnu" CGO_LDFLAGS="--sysroot=/ -L/usr/lib/aarch64-linux-gnu" PKG_CONFIG_LIBDIR=/usr/lib/aarch64-linux-gnu/pkgconfig go build -buildvcs=false -trimpath -ldflags "-s -w -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${VERSION}" -o "dist/linux/gosentry-${VERSION}-linux-arm64" ./cmd/gosentry'
}

build_windows_amd64() {
    run_in_builder 'mkdir -p dist/windows && x86_64-w64-mingw32-windres -O coff -o cmd/gosentry/rsrc_windows_amd64.syso packaging/windows/gosentry.rc && CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags "-s -w -H=windowsgui -X gitea.mixdep.ru/mix/gosentry/src/app.Version=${VERSION}" -o "dist/windows/gosentry-${VERSION}-windows-amd64.exe" ./cmd/gosentry'
}

mapfile -t targets < <(choose_targets "$@" | normalize_targets | awk '!seen[$0]++')
if [ "${#targets[@]}" -eq 0 ]; then
    echo "No build targets selected." >&2
    exit 1
fi

echo "Building Docker builder image: $tag"
docker build -f Dockerfile -t "$tag" .

for target in "${targets[@]}"; do
    echo "Building $target..."
    case "$target" in
        linux-amd64)
            build_linux_amd64
            ;;
        linux-arm64)
            build_linux_arm64
            ;;
        windows-amd64)
            build_windows_amd64
            ;;
    esac
done

echo "Built release artifacts:"
find dist/linux dist/windows -maxdepth 1 -type f -print 2>/dev/null || true
