# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go module files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build all binaries inside cmd/ directory (server, migrate, create-admin, etc) into /out/
RUN CGO_ENABLED=0 GOOS=linux go build -tags go_json -o /out/ ./cmd/...

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies including NGINX proxy
RUN apk add --no-cache ca-certificates tzdata nginx

# Copy all compiled binaries from builder directly into system PATH
COPY --from=builder /out/ /usr/local/bin/

# Copy migrations for convenience
COPY --from=builder /app/migrations /app/migrations

# Create uploads directory and inject NGINX configs + startup scripts
RUN mkdir -p /app/uploads
COPY nginx.conf /app/nginx.conf
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh

# The container explicitly exposes 8080 (handled natively by NGINX internally)
EXPOSE 8080

# Sequence NGINX reverse proxy orchestrating Go binary natively
CMD ["/app/start.sh"]
