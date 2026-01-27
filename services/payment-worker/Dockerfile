# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o payment-worker ./services/payment-worker/cmd

# Runtime stage
FROM alpine:3.19

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/payment-worker .

# Copy migrations
COPY --from=builder /app/shared/database/migrations ./migrations

# Create non-root user
RUN adduser -D -u 1000 appuser
USER appuser

ENTRYPOINT ["./payment-worker"]
