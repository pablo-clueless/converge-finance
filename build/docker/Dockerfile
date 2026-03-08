# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/converge \
    ./cmd/converge

# Final stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 converge && \
    adduser -u 1000 -G converge -s /bin/sh -D converge

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/converge /app/converge

# Copy migrations
COPY --from=builder /app/migrations /app/migrations

# Change ownership
RUN chown -R converge:converge /app

# Switch to non-root user
USER converge

# Expose ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Default command
CMD ["./converge", "serve"]
