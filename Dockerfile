# Stage 1: Build the Go binary
FROM golang:1.26 AS builder
WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bot ./cmd/bot

# Stage 2: Runtime environment with Podman, bash, curl, and git
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    curl \
    git \
    podman \
    ca-certificates \
    fuse-overlayfs \
    iptables \
    procps \
    openssl \
    && rm -rf /var/lib/apt/lists/*

# Configure Podman storage for containerized execution
RUN mkdir -p /etc/containers && \
    printf '[storage]\ndriver = "overlay"\nrunroot = "/run/containers/storage"\ngraphroot = "/var/lib/containers/storage"\n[storage.options.overlay]\nmount_program = "/usr/bin/fuse-overlayfs"\n' > /etc/containers/storage.conf

WORKDIR /app

COPY --from=builder /build/bot /app/bot
COPY configs/ /app/configs/

ENTRYPOINT ["/app/bot"]
CMD ["--config", "/app/configs/config.yaml"]
