# syntax=docker/dockerfile:1

FROM golang:1.22-bookworm AS builder
WORKDIR /src

# Go modules and local replacements
COPY go.mod ./
COPY modernc.org ./modernc.org

# Copy the remaining source code
COPY . .

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ENV CGO_ENABLED=0

RUN GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -trimpath \
    -ldflags="-s -w" \
    -tags timetzdata \
    -o /out/scheduler \
    ./cmd/scheduler

FROM debian:bookworm-slim AS runner

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

RUN useradd --system --create-home --home-dir /nonexistent --shell /usr/sbin/nologin scheduler

WORKDIR /app
COPY --from=builder /out/scheduler /usr/local/bin/scheduler

USER scheduler
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/scheduler"]
