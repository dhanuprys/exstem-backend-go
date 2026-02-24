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

# Build with go_json tag for fast serialization
RUN CGO_ENABLED=0 GOOS=linux go build -tags go_json -o /bin/server ./cmd/server

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /bin/server /app/server

# Copy migrations for convenience
COPY --from=builder /app/migrations /app/migrations

# Create uploads directory
RUN mkdir -p /app/uploads

EXPOSE 8080

CMD ["/app/server"]
