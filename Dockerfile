# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for version information
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=$(git describe --tags --always --dirty) -s -w" -o httprunner

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/httprunner .

# Make it executable
RUN chmod +x ./httprunner

# Create a directory for HTTP request files
RUN mkdir -p /requests

# Set the binary as the entrypoint
ENTRYPOINT ["./httprunner"]

# Default command shows help
CMD ["--help"]