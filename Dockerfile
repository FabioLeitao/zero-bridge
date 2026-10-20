# Multi-stage Dockerfile for zero-bridge
# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /build

# The core module has no third-party requirement and no go.sum.
COPY bridge/go.mod ./bridge/

# Copy source
COPY bridge/ ./bridge/

# Build static binary
RUN cd bridge && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE}" \
    -o /zero-bridge ./cmd/zero-bridge

# Runtime stage
FROM alpine:3.20 AS runtime

# Install runtime dependencies (ca-certificates for TLS, tzdata for timezone)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    dumb-init

# Create non-root user
ARG HOME_DIR=/var/lib/zerobridge
RUN addgroup -g 10001 -S zerobridge && \
    adduser -u 10001 -S -D -H -h ${HOME_DIR} -G zerobridge zerobridge

# Create directories
RUN mkdir -p ${HOME_DIR}/.local/state/zero-bridge /tmp/zerobridge && \
    chown -R zerobridge:zerobridge ${HOME_DIR} /tmp/zerobridge && \
    chmod 700 ${HOME_DIR}/.local/state/zero-bridge && \
    chmod 1777 /tmp/zerobridge

# Copy binary from builder
COPY --from=builder /zero-bridge /usr/local/bin/zero-bridge

RUN chmod 755 /usr/local/bin/zero-bridge

# Switch to non-root user
USER zerobridge

# Read-only root, cap_drop, and no-new-privileges are set in docker-compose.yml.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["zero-bridge", "--healthcheck", "http://127.0.0.1:8765/health"]

# Expose port (documentation only; actual bind is loopback)
EXPOSE 8765

# Default command
ENTRYPOINT ["dumb-init", "--"]
CMD ["zero-bridge", "--listen", "127.0.0.1:8765"]