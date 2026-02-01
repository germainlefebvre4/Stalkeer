# Multi-stage Dockerfile for Stalkeer
# Stage 1: Builder
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /build/stalkeer \
    ./cmd/...

# Stage 2: Runtime
FROM alpine:3.21

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 stalkeer && \
    adduser -D -u 1000 -G stalkeer stalkeer

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/stalkeer /app/stalkeer

# Create necessary directories with proper permissions
RUN mkdir -p /app/data /app/config /app/m3u_playlist && \
    chown -R stalkeer:stalkeer /app

# Switch to non-root user
USER stalkeer

# Expose ports
# 8080: API server
# 8081: Admin server (if needed)
EXPOSE 8080 8081

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
CMD ["/app/stalkeer", "server"]
