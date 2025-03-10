# Build stage
FROM --platform=linux/amd64 golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application specifically for AMD64
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -o server ./cmd/server

# Final stage
FROM --platform=linux/amd64 alpine:3.21.3

# Install required dependencies for SQLite
RUN apk --no-cache add ca-certificates tzdata sqlite

# Create a non-root user and group with ID 2000
RUN addgroup -g 2000 -S invoice && adduser -u 2000 -S invoice -G invoice

WORKDIR /app

# Copy the binary from the builder stage directly with the correct name
COPY --from=builder /app/server .

# Copy templates
COPY --from=builder /app/internal/templates /app/internal/templates

# Create directory with correct permissions
RUN mkdir -p /app/data/images && \
    chown -R 2000:2000 /app/data && \
    chmod -R 755 /app/data && \
    chmod +x /app/server

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV DATA_DIR=/app/data
ENV LOG_LEVEL="INFO"

# Volume for persistent data
VOLUME ["/app/data"]

# Switch to non-root user using numeric ID for Kubernetes compatibility
USER 2000

# Run the application with absolute path
CMD ["/app/server"]

LABEL org.opencontainers.image.source=https://github.com/0dragosh/simple-invoice
