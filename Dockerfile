FROM golang:1.22-bookworm

# Fyne links against native desktop libraries, so the container must include a C
# compiler plus OpenGL/X11 headers. --no-install-recommends keeps the image from
# pulling in unrelated desktop packages that are not needed for compilation.
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        gcc \
        libgl1-mesa-dev \
        xorg-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy module files first so Docker can cache downloaded dependencies while the
# application source changes. This makes repeated local builds much faster.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is required by Fyne. The first Linux package target is linux/amd64; other
# architectures can be added later as separate, explicit build targets.
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# -trimpath removes host paths from the binary, and -s -w strips symbol/debug
# tables to keep the produced desktop executable smaller.
RUN go build -trimpath -ldflags "-s -w" -o /out/pysentry ./cmd/pysentry
