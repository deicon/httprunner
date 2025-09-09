# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for version information
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the applications
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=$(git describe --tags --always --dirty) -s -w" -o httprunner ./cmd/httprunner
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=$(git describe --tags --always --dirty) -s -w" -o harparser ./cmd/harparser

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /httprunner/

# Copy the binary from builder stage
COPY --from=builder /app/httprunner .
COPY --from=builder /app/harparser .

# Make it executable
RUN chmod +x ./httprunner
RUN chmod +x ./harparser

# Create a directory for HTTP request files
RUN mkdir -p /requests

# Set the binary as the entrypoint
ENTRYPOINT []

# Default command shows help
CMD ["./httprunner", "--help"]