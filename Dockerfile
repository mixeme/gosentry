FROM golang:1.22-bookworm

# Fyne links against native desktop libraries, so the container must include a C
# compiler plus OpenGL/X11 headers. --no-install-recommends keeps the image from
# pulling in unrelated desktop packages that are not needed for compilation.
RUN apt-get update && \
    dpkg --add-architecture arm64 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        gcc \
        gcc-aarch64-linux-gnu \
        gcc-mingw-w64-x86-64 \
        binutils-mingw-w64-x86-64 \
        pkg-config \
        libgl1-mesa-dev \
        xorg-dev \
        libgl1-mesa-dev:arm64 \
        xorg-dev:arm64 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy module files first so Docker can cache downloaded dependencies while the
# application source changes. This makes repeated local builds much faster.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is required by Fyne. This builder produces the release artifacts from
# Linux: Linux amd64, Linux arm64, and a Windows amd64 binary cross-compiled with
# MinGW. The Windows resource is generated inside the container so Explorer still
# sees the application icon.
RUN version="$(sed -n 's/^var Version = "\(.*\)"/\1/p' src/core/version.go)" && \
    version="${version:-0.0.0-dev}" && \
    mkdir -p /out/linux /out/windows && \
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
        go build -trimpath -ldflags "-s -w -X github.com/pysentry/pysentry/src/core.Version=${version}" \
        -o "/out/linux/pysentry-${version}-linux-amd64" ./cmd/pysentry && \
    CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 \
        PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig \
        go build -trimpath -ldflags "-s -w -X github.com/pysentry/pysentry/src/core.Version=${version}" \
        -o "/out/linux/pysentry-${version}-linux-arm64" ./cmd/pysentry && \
    x86_64-w64-mingw32-windres -O coff -o cmd/pysentry/rsrc_windows_amd64.syso packaging/windows/pysentry.rc && \
    CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
        go build -trimpath -ldflags "-s -w -H=windowsgui -X github.com/pysentry/pysentry/src/core.Version=${version}" \
        -o "/out/windows/pysentry-${version}-windows-amd64.exe" ./cmd/pysentry
