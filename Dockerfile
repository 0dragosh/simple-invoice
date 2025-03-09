# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -o simple-invoice ./cmd/server

# Final stage
FROM alpine:3.21.3

# Install required dependencies for SQLite
RUN apk --no-cache add ca-certificates tzdata sqlite

# Create a non-root user and group with ID 2000
RUN addgroup -g 2000 -S invoice && adduser -u 2000 -S invoice -G invoice

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/simple-invoice .

# Copy templates
COPY --from=builder /app/internal/templates /app/internal/templates

# Create data directory
RUN mkdir -p /app/data/images /app/data/pdfs && \
    chown -R invoice:invoice /app && \
    chmod -R 755 /app/data

# Expose port
EXPOSE 8080

# Set environment variables
ENV PORT=8080
ENV DATA_DIR=/app/data
ENV LOG_LEVEL="INFO"

# Volume for persistent data
VOLUME ["/app/data"]

# Switch to non-root user
USER invoice

# Run the application
CMD ["./simple-invoice"]

LABEL org.opencontainers.image.source=https://github.com/0dragosh/simple-invoice
