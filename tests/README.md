# Test Files Organization

This directory contains all `.http` test files for the httprunner project, organized by purpose.

## Directory Structure

```
tests/
├── e2e/                    # End-to-end integration tests (run in CI/CD)
│   ├── comprehensive-test.http    # Main test suite (394 lines)
│   └── toxiproxy-demo.http        # Network simulation tests (91 lines)
│
├── unit/                   # Feature-specific unit tests
│   ├── assertions.http     # Consolidated assertion tests (161 lines)
│   └── scripting.http      # Script-only scenarios (19 lines)
│
└── examples/              # Documentation & showcase files
    ├── context-demo.http              # Context API demo (35 lines)
    ├── demo.http                      # Basic check examples (50 lines)
    ├── metrics-showcase.http          # Metrics features (211 lines)
    ├── scripting-demo.http            # Advanced scripting (123 lines)
    ├── external-node-runtime/         # Node.js runtime example
    │   └── test-external.http
    └── k6/                            # K6 integration example
        └── loadtest.http
```

## Test Categories

### E2E Tests (`e2e/`)
End-to-end integration tests that validate complete workflows against real services.
- **comprehensive-test.http**: Main test suite used by CI/CD pipeline (Docker Compose)
- **toxiproxy-demo.http**: Network condition simulation (latency, errors, timeouts)

### Unit Tests (`unit/`)
Feature-specific tests focusing on individual capabilities.
- **assertions.http**: Tests for `client.check()` and `client.assert()` functionality
- **scripting.http**: Pre/post-request script execution tests

### Examples (`examples/`)
Documentation and demonstration files showcasing features.
- **metrics-showcase.http**: Complete metrics API demonstration
- **scripting-demo.http**: Advanced scripting features (@BeforeUser, @BeforeIteration, etc.)
- **context-demo.http**: Context API (userId, iterationId) usage
- **demo.http**: Basic check examples for documentation
- **external-node-runtime/**: Node.js runtime with npm dependencies
- **k6/**: K6 load testing integration

## Running Tests

### Via Docker Compose (E2E)
```bash
./run-tests.sh
# OR
docker compose up --build httprunner
```

### Direct Execution
```bash
# E2E test
./build/httprunner -f tests/e2e/comprehensive-test.http -output results

# Unit test
./build/httprunner -f tests/unit/assertions.http -output results

# Example/demo
./build/httprunner -f tests/examples/metrics-showcase.http -output results
```

### External Node Runtime Example
```bash
npm install --prefix tests/examples/external-node-runtime
./build/httprunner \
  --experimental-node-runtime \
  -f tests/examples/external-node-runtime/test-external.http \
  -report console
```

## Notes

- **testapi/** directory (at project root) contains test files specific to the local test API server development and is kept separate.
- E2E tests require Docker services (testapi, toxiproxy) to be running.
- Results and reports are generated in the `results/` or `reports/` directory (not tracked in git).
