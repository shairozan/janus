# syntax=docker/dockerfile:1

# Build stage
FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Set GOTOOLCHAIN to auto to allow Go to download the required version
ENV GOTOOLCHAIN=auto

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the license-server binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o license-server ./cmd/license-server

# Runtime stage
FROM scratch

# Copy timezone data and CA certificates from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the binary from builder
COPY --from=builder /build/license-server /license-server

# Copy migrations
COPY --from=builder /build/internal/license/db/migrations /migrations

# Expose default port (can be overridden)
EXPOSE 8080

# Run as non-root user
USER 65534:65534

ENTRYPOINT ["/license-server"]
CMD ["serve"]
