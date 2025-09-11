#!/bin/bash

set -e

echo "🚀 Starting HTTP Runner Test Environment..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Clean up function
cleanup() {
    print_status "Cleaning up containers..."
    docker-compose down -v 2>/dev/null || true
    docker system prune -f 2>/dev/null || true
}

# Set trap to cleanup on script exit
trap cleanup EXIT

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    print_error "Docker is not running. Please start Docker and try again."
    exit 1
fi

# Check if Docker Compose is available
if ! command -v docker-compose > /dev/null 2>&1; then
    print_error "docker-compose is not installed. Please install docker-compose and try again."
    exit 1
fi

print_status "Building and starting services..."

# Build and start services
docker-compose up --wait --wait-timeout 300 --build --remove-orphans -d testapi toxiproxy

print_status "Waiting for services to be healthy..."

print_status "Services status:"
docker-compose ps

# Create reports directory if it doesn't exist
mkdir -p reports

print_status "Starting comprehensive tests..."

# Run the HTTP Runner tests
if docker-compose up --build httprunner; then
    print_success "✅ All tests completed successfully!"
    
    print_status "Test results are available in the reports/ directory:"
    
    print_status "Container logs:"
    echo "=== Test API Logs ==="
    docker-compose logs testapi
    echo ""
    echo "=== HTTP Runner Logs ==="
    docker-compose logs httprunner
    
else
    print_error "❌ Tests failed!"
    
    print_status "Container logs for debugging:"
    echo "=== Test API Logs ==="
    docker-compose logs testapi
    echo ""
    echo "=== Toxiproxy Logs ==="
    docker-compose logs toxiproxy
    echo ""
    echo "=== HTTP Runner Logs ==="
    docker-compose logs httprunner
    
    exit 1
fi

print_success "🎉 Test environment completed successfully!"
print_status "You can view the HTML report by opening reports/test-results.html in your browser"