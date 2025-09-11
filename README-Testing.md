# HTTP Runner Docker Compose Test Environment

This directory contains a complete Docker Compose setup for testing the HTTP Runner with a sample REST API, network simulation via Toxiproxy, and comprehensive test scenarios.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Runner   │    │   Toxiproxy     │    │   Test API      │
│                 │    │   (Network      │    │   (Go REST      │
│   - Executes    │────┤   Simulator)    │────┤   Service)      │
│     tests       │    │   - Latency     │    │   - Users       │
│   - Metrics     │    │   - Errors      │    │   - Products    │
│   - Reports     │    │   - Timeouts    │    │   - Health      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Services

### 1. Test API Server (`testapi`)
- **Port**: 8080
- **Language**: Go (using only standard library)
- **Features**:
  - User CRUD operations (`/api/users`)
  - Product management (`/api/products`) 
  - Health check (`/health`)
  - Slow endpoint simulation (`/api/slow`)
  - Random error simulation (`/api/random-error`)
  - Echo service (`/api/echo`)

### 2. Toxiproxy (`toxiproxy`)
- **API Port**: 8474
- **Proxy Port**: 8081 (proxies to testapi:8080)
- **Purpose**: Network condition simulation
- **Features**:
  - Latency injection
  - Connection timeouts
  - Error rate simulation
  - Bandwidth limiting

### 3. HTTP Runner (`httprunner`)
- **Purpose**: Execute comprehensive tests
- **Features**:
  - Template support with environment variables
  - JavaScript scripting (pre/post request)
  - Metrics collection and analysis
  - Multiple report formats (HTML, JSON, CSV)
  - Validation checks

## Quick Start

### Prerequisites
- Docker and Docker Compose installed
- Git (for CI/CD integration)

### Run Tests Locally

```bash
# Make the script executable (if not already)
chmod +x run-tests.sh

# Run the complete test suite
./run-tests.sh
```

This will:
1. Build all services from source
2. Start the test API and Toxiproxy
3. Wait for services to be healthy
4. Execute comprehensive tests
5. Generate reports in `reports/` directory
6. Clean up containers

### Manual Testing

```bash
# Start services only
docker-compose up --build -d testapi toxiproxy

# Run specific test file
docker-compose run --rm httprunner ./httprunner -u 3 -i 5 -f tests/comprehensive-test.http

# Run with Toxiproxy simulation
docker-compose run --rm httprunner ./httprunner -u 2 -i 3 -f tests/toxiproxy-demo.http

# View logs
docker-compose logs testapi
docker-compose logs toxiproxy

# Clean up
docker-compose down -v
```

## Test Files

### `tests/comprehensive-test.http`
Complete test suite demonstrating all HTTP Runner features:
- ✅ Health checks
- ✅ CRUD operations (Users, Products)
- ✅ Template variable usage
- ✅ Pre/post request JavaScript scripting
- ✅ Metrics collection and analysis
- ✅ Validation checks with `client.check()`
- ✅ Global variable management
- ✅ Performance monitoring
- ✅ Error handling

### `tests/toxiproxy-demo.http`
Network condition simulation:
- ✅ Normal proxy operation
- ✅ Latency injection (500ms delay)
- ✅ Performance impact measurement
- ✅ Toxic management via API

## Environment Variables

The following environment variables are available in test files:

- `BASEURL`: `http://testapi:8080` - Direct API access
- `TOXIPROXY_URL`: `http://toxiproxy:8081` - Proxied API access  
- `API_VERSION`: `v1` - API version for templating

## Reports

After running tests, check the `reports/` directory for:

- **`test-results.html`**: Interactive HTML report with charts
- **`test-results.json`**: Structured JSON data for automation
- **`test-results.csv`**: CSV format for spreadsheet analysis

## CI/CD Integration

### GitHub Actions

The included workflow (`.github/workflows/integration-test.yml`) will:
- Run on push to `main`, `develop`, or `feat/*` branches
- Execute the full test suite
- Upload test reports as artifacts
- Cache Docker layers for faster builds

### Usage in Your Pipeline

```yaml
- name: Run HTTP Runner Integration Tests
  run: |
    chmod +x run-tests.sh
    ./run-tests.sh

- name: Upload Test Reports
  uses: actions/upload-artifact@v3
  with:
    name: http-runner-reports
    path: reports/
```

## Advanced Testing Scenarios

### Load Testing
```bash
# High concurrency test
docker-compose run --rm httprunner \
  ./httprunner -u 50 -i 20 -d 100 -f tests/comprehensive-test.http \
  --html-report reports/load-test.html

# Stress test with Toxiproxy
# 1. Start services
docker-compose up -d testapi toxiproxy

# 2. Add bandwidth limiting
curl -X POST http://localhost:8474/proxies/testapi_proxy/toxics \
  -H "Content-Type: application/json" \
  -d '{"name":"bandwidth","type":"bandwidth","toxicity":1.0,"attributes":{"rate":1000}}'

# 3. Run tests
docker-compose run --rm httprunner \
  ./httprunner -u 10 -i 50 -f tests/comprehensive-test.http
```

### Network Failure Simulation
```bash
# Simulate connection timeouts
curl -X POST http://localhost:8474/proxies/testapi_proxy/toxics \
  -H "Content-Type: application/json" \
  -d '{"name":"timeout","type":"timeout","toxicity":0.3,"attributes":{"timeout":1000}}'
```

## Customization

### Adding New Endpoints
1. Edit `testapi/main.go` to add new handlers
2. Update `tests/comprehensive-test.http` with new test cases
3. Rebuild with `docker-compose build testapi`

### Custom Test Scenarios
1. Create new `.http` files in the `tests/` directory
2. Use the same format as existing files
3. Reference environment variables with `{{.VARIABLE_NAME}}`

### Toxiproxy Configuration
Edit `toxiproxy-config.json` to:
- Add more proxies
- Change upstream targets  
- Pre-configure toxics

## Troubleshooting

### Services Not Starting
```bash
# Check service health
docker-compose ps

# View logs
docker-compose logs testapi
docker-compose logs toxiproxy

# Restart services
docker-compose restart
```

### Tests Failing
```bash
# Run with verbose logging
docker-compose run --rm httprunner \
  ./httprunner -u 1 -i 1 -f tests/comprehensive-test.http -v

# Check API directly
curl http://localhost:8080/health
curl http://localhost:8081/health  # via proxy
```

### Network Issues
```bash
# Check Docker network
docker network ls
docker network inspect dockercompose-testing_test-network

# Test connectivity
docker-compose run --rm httprunner wget -O- http://testapi:8080/health
```

## Performance Expectations

With this setup, you should expect:
- **Direct API calls**: < 50ms response time
- **Proxied calls**: < 100ms response time  
- **With 500ms latency toxic**: ~550ms response time
- **Test suite completion**: 30-60 seconds
- **Memory usage**: < 500MB total for all services

This test environment provides a comprehensive way to validate HTTP Runner functionality across various network conditions and API scenarios.