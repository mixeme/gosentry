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
        libc6-dev \
        gcc-aarch64-linux-gnu \
        libc6-dev-arm64-cross \
        linux-libc-dev-arm64-cross \
        gcc-mingw-w64-x86-64 \
        binutils-mingw-w64-x86-64 \
        pkg-config \
        libgl1-mesa-dev \
        xorg-dev \
        libgl1-mesa-dev:arm64 \
        libx11-dev:arm64 \
        libxcursor-dev:arm64 \
        libxrandr-dev:arm64 \
        libxinerama-dev:arm64 \
        libxi-dev:arm64 \
        libxxf86vm-dev:arm64 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy module files first so Docker can cache downloaded dependencies while the
# application source changes. The release script later mounts the live repository
# over /src, but the module cache remains in the image and keeps repeated builds
# faster.
COPY go.mod go.sum ./
RUN go mod download

# The image intentionally stops here. Artifact build commands live in
# scripts/build-release-linux.sh so a developer can choose targets interactively
# without rebuilding this environment image for every selection.
