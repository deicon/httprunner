# httprunner

A Go-based command-line tool for running multiple, parallel HTTP requests.

## Installation

### Using `go install`

Prerequisites: Go 1.20+ and a working internet connection.

Install the CLI tools directly from the module:

```bash
go install github.com/deicon/httprunner/cmd/httprunner@latest
go install github.com/deicon/httprunner/cmd/harparser@latest
```

Ensure your install bin directory is on PATH:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"   # if GOBIN not set
```

Verify installation and version:

```bash
httprunner -version
harparser -version
```

Notes:
- Replace `@latest` with a specific tag (e.g., `@vX.Y.Z`) to pin a version.
- To upgrade later, rerun the same `go install ...@latest` commands.

### Homebrew (macOS/Linux)

Once a release is published, prebuilt bottles are provided via Formula files in this repo. Install directly from the raw formula URLs:

```bash
# httprunner
brew install --formula https://raw.githubusercontent.com/deicon/httprunner/refs/heads/main/Formula/httprunner.rb

# harparser
brew install --formula https://raw.githubusercontent.com/deicon/httprunner/refs/heads/main/Formula/harparser.rb
```

Notes:
- Homebrew supports macOS and Linux; Windows users can download the ZIP asset from Releases.
- The CI release job updates these formulas with per-platform URLs and SHA256 checksums automatically on every tagged release.

## Project Structure

```
github.com/deicon/httprunner/
├── main.go              # Entry point and CLI argument parsing
├── http/
│   └── requests.go      # HTTP request data structure
├── parser/
│   ├── parser.go        # .http file parsing logic
│   └── parser_test.go   # Parser tests
├── runner/
│   └── runner.go        # Request execution and concurrency management
├── docs/
│   └── specs/
│       └── requirements.md  # Project requirements and specifications
└── requests.http        # Sample HTTP requests file
```

## Usage

```bash
./httprunner -u <virtualuser> -i <iterations> -d <delay> -f <file>
```

### Parameters
- `-u n`: Number of parallel virtualusers (default: 1)
- `-i n`: Number of iterations (default: 1)  
- `-r n`: Runtime in seconds. Overwrites iterations
- `-d n`: Delay between iterations in milliseconds (default: 0)
- `-f filename`: .http file containing HTTP requests (required)
- `-e filename`: .env file containing environment variables (optional)

#### Reporting and Output Parameters
- `-report format`: Report format: console, html, csv, json (default: html)
- `-output directory`: Output directory for streaming results and reports (enables streaming mode)
- `-detail level`: Report detail level: summary, goroutine, iteration, full (default: full)
- `-stream`: Enable streaming mode to reduce memory usage (default: true, requires -output)

### Examples

#### Basic usage (with new defaults)
```bash
# Simple run with defaults (streaming=true, report=html, detail=full)
./httprunner -f requests.http -output test_results

# Traditional usage
./httprunner -t 20 -i 10 -d 1000 -f requests.http -output results

# Console output with summary detail
./httprunner -f requests.http -report console -detail summary -stream false

# CSV export with full detail
./httprunner -f requests.http -report csv -output reports
```

## HTTP Request File Format

Requests are separated by `###` and follow this format:

```
### 
# @name <Request Name>
<HTTP_VERB> <URL>
<Header-Name>: <Header-Value>
<Header-Name>: <Header-Value>

<JSON_BODY>

> {%
<JavaScript_Code>
%}

### 
<HTTP_VERB> <URL>

```

### Template Support

URLs, headers, and request bodies support Go template syntax using `{{.VARIABLE_NAME}}`. All environment variables are available as template variables.

### JavaScript Scripting

JavaScript code can be embedded using `> {%` and `%}` blocks in two locations:

1. **Pre-request scripts**: Placed before the HTTP verb/URL line (separated by blank lines). These scripts execute before the request is sent and have access to:
   - `client.global.set(key, value)`: Store values in global variables
   - `client.global.get(key)`: Retrieve values from global variables

2. **Post-request scripts**: Placed after the request body. These scripts execute after the request is sent and have access to:
   - `response.body`: The response body (parsed as JSON if valid, otherwise as string)
   - `client.global.set(key, value)`: Store values in global variables
   - `client.global.get(key)`: Retrieve values from global variables
   - `client.check(name, checkHandler, failureMessage)`: Perform validation checks (see Check Functions section)
   - `client.metrics`: Access to performance and execution metrics (see Metrics Access section)
   - `sleep(millis)`: Sleep execution for millis
   - `assert(function() -> bool, message, status_code )`: assert using function returning bool and fail request if false

### Example requests.http

#### Basic requests (backward compatible):
```
### 
POST http://localhost:8080/api/v3/tarifrechner
Content-Type: application/json

{
  "tarifrechnerModus": {
    "modus": "TARIFRECHNER",
    "mandant": "ORGA",
    "haushaltsId": 48296349
  },
  "kundennummern": [87468640],
  "produktKonfigurationId": "investmentanlage",
  "vertragsId": 7007787476
}

### 
GET http://example.com
Accept: application/json

###
```

#### Template and scripting example:
```
### 
# @name Ressource anlegen
> {%
// Pre-request script: Set up dynamic values before the request
client.global.set("currentTimestamp", Date.now().toString())
client.global.set("requestId", "req_" + Math.random().toString(36).substring(7))
%}

POST {{.BASEURL}}/api/{{.APIVERSION}}/ressource
Authorization: Bearer {{.TOKEN}}
Content-Type: application/json
X-Request-ID: {{.requestId}}

{
"modus": {
"modus": "{{.MODUS}}",
"mandant": "ORGA",
"partnerID": "{{.PARTNERID}}"
},
"timestamp": "{{.currentTimestamp}}"
}

> {%
// Post-request script: Extract values from response
var jsonData = response.body
client.global.set("ID", jsonData.id)
%}

### 
# @name Get Tarifrechner
GET {{.BASEURL}}/api/{{.APIVERSION}}/ressource/{{.ID}}
Authorization: Bearer {{.TOKEN}}
Accept: application/json

###
```

### Metrics Access

The `client.metrics` object provides access to real-time performance and execution metrics during script execution. This enables performance monitoring, trend analysis, and data-driven test validation.

#### Available Metrics

httprunner automatically collects these built-in metrics:

- **http_reqs** (Counter): Total number of HTTP requests made
- **http_req_duration** (Histogram): Duration of HTTP requests in milliseconds
- **http_req_failed** (Rate): Rate of failed HTTP requests (0.0 to 1.0)
- **iterations** (Counter): Number of completed iterations
- **checks** (Rate): Success rate of validation checks

#### Metrics API

##### `client.metrics.getCurrent(metricName)`
Returns the latest/current value for a metric:

```javascript
var currentDuration = client.metrics.getCurrent("http_req_duration");
var totalRequests = client.metrics.getCurrent("http_reqs");
console.log("Request #" + totalRequests + " took " + currentDuration + "ms");
```

##### `client.metrics.get(metricName)`
Returns a complete metric summary object with statistical data:

```javascript
var durationStats = client.metrics.get("http_req_duration");
if (durationStats) {
    console.log("Average: " + durationStats.average + "ms");
    console.log("Min: " + durationStats.min + "ms");
    console.log("Max: " + durationStats.max + "ms");
    console.log("P95: " + durationStats.p95 + "ms");
}
```

**Metric Summary Object Properties:**
- `name`: Metric name
- `type`: Metric type ("counter", "histogram", "rate")
- `count`: Number of data points
- `sum`: Total sum of all values
- `average`: Average value
- `min`: Minimum value
- `max`: Maximum value
- `p50`: 50th percentile
- `p90`: 90th percentile
- `p95`: 95th percentile
- `p99`: 99th percentile
- `latest_value`: Most recent value

##### `client.metrics.getAll()`
Returns all metrics as an object:

```javascript
var allMetrics = client.metrics.getAll();
Object.keys(allMetrics).forEach(function(name) {
    var metric = allMetrics[name];
    console.log(name + ": " + metric.type + " (" + metric.count + " samples)");
});
```

#### Performance Monitoring Examples

##### Response Time Validation
```javascript
> {%
// Check individual request performance
var duration = client.metrics.getCurrent("http_req_duration");
client.check("Response Time", function() {
    return duration < 2000; // Under 2 seconds
}, "Request should complete within 2 seconds");

// Check performance trends
var durationStats = client.metrics.get("http_req_duration");
if (durationStats && durationStats.count >= 5) {
    client.check("Average Performance", function() {
        return durationStats.average < 1500;
    }, "Average response time should be under 1.5 seconds");
}
%}
```

##### Error Rate Monitoring
```javascript
> {%
// Monitor failure rates
var failureStats = client.metrics.get("http_req_failed");
var totalReqs = client.metrics.getCurrent("http_reqs");

if (failureStats && totalReqs > 10) {
    var failureRate = failureStats.average * 100;
    console.log("Current failure rate: " + failureRate.toFixed(2) + "%");
    
    client.check("Error Rate Threshold", function() {
        return failureRate < 5; // Less than 5%
    }, "Error rate should be under 5%");
}
%}
```

##### Throughput Analysis
```javascript
> {%
// Calculate throughput
var durationStats = client.metrics.get("http_req_duration");
if (durationStats) {
    var avgDurationSec = durationStats.average / 1000;
    var throughput = 1 / avgDurationSec;
    console.log("Throughput: " + throughput.toFixed(2) + " req/s");
    
    client.check("Throughput Target", function() {
        return throughput >= 2.0; // At least 2 requests per second
    }, "Should meet minimum throughput requirement");
}
%}
```

##### Performance Regression Detection
```javascript
> {%
// Store baseline for comparison
var currentDuration = client.metrics.getCurrent("http_req_duration");
var baseline = client.global.get("performance_baseline");

if (!baseline) {
    client.global.set("performance_baseline", currentDuration);
} else {
    var regression = ((currentDuration - baseline) / baseline) * 100;
    console.log("Performance change: " + regression.toFixed(1) + "%");
    
    client.check("No Performance Regression", function() {
        return Math.abs(regression) < 50; // Within 50%
    }, "Performance should not regress significantly");
}
%}
```

#### Complete Metrics Dashboard Example

See `examples/metrics-showcase.http` for a comprehensive example demonstrating:
- Performance baseline establishment
- Load pattern analysis
- Error rate monitoring  
- Throughput calculations
- Comprehensive metrics reporting

### Check Functions

The `client.check()` function allows you to perform validation checks on HTTP responses. These checks are tracked and reported in the final test results.

#### Syntax
```javascript
client.check(name, checkHandler, failureMessage)
```

#### Parameters
- **name** (string): A unique identifier for the check
- **checkHandler** (function): A function that returns `true` for success, `false` for failure
- **failureMessage** (string): Message displayed when the check fails

#### Examples

```
### 
# @name API Response Validation
POST {{.BASEURL}}/api/users
Content-Type: application/json

{
  "username": "testuser",
  "email": "test@example.com"
}

> {%
// Post-request validation checks
client.check("Status Code Check", function() {
    return response.body.status === "success";
}, "API should return success status");

client.check("User ID Present", function() {
    return response.body.user && response.body.user.id;
}, "Response should contain user ID");

client.check("Valid Email Format", function() {
    var email = response.body.user.email;
    return email && email.includes("@");
}, "Email should be in valid format");

// Store user ID for subsequent requests
if (response.body.user && response.body.user.id) {
    client.global.set("userId", response.body.user.id);
}
%}

### 
# @name Get User Details
GET {{.BASEURL}}/api/users/{{.userId}}
Authorization: Bearer {{.TOKEN}}

> {%
client.check("User Details Retrieved", function() {
    return response.body && response.body.id == client.global.get("userId");
}, "Should retrieve correct user details");

client.check("Response Time Check", function() {
    var duration = client.metrics.getCurrent("http_req_duration");
    return duration && duration < 2000; // Should be under 2 seconds
}, "Response time should be under 2 seconds");
%}

###
```

#### Check Results in Reports

Check results are included in all report formats:

- **Console**: Shows check summary with total/successful/failed counts and breakdown by check name
- **HTML**: Displays check metrics in the summary section and detailed results
- **JSON**: Includes `checkSummaries` object with detailed check statistics
- **CSV**: Individual check results are included in request details

Failed checks do not stop test execution but are counted and reported for analysis.

## Building and Testing

```bash
# Build the project
go build

# Run tests
go test ./parser

# Run with sample requests (using new defaults)
./httprunner -f requests.http -output test_results

# Run without streaming for console output
./httprunner -f requests.http -stream=false
```

## Features

### Streaming Mode
- **Memory efficient**: Processes results as they come in, reducing memory usage for large workloads
- **Real-time results**: Results are written to files immediately as requests complete
- **Automatic activation**: Automatically enabled for workloads > 10,000 total operations
- **Output formats**: Raw results in JSONL format + formatted reports

### Multiple Report Formats
- **HTML**: Rich, interactive reports with charts and detailed statistics (default)
- **Console**: Simple text output for terminal viewing
- **CSV**: Spreadsheet-compatible format for data analysis
- **JSON**: Machine-readable format for integration with other tools

### Report Detail Levels
- **Summary**: High-level overview of performance metrics
- **Goroutine**: Per-goroutine breakdown of performance
- **Iteration**: Detailed per-iteration results
- **Full**: Complete detailed breakdown including all request/response data (default)

## Architecture

- **main.go**: Handles CLI flag parsing and coordinates the parser and runner
- **http/requests.go**: Defines the Request struct containing Name, Verb, URL, Headers, Body, and Script
- **parser/parser.go**: Parses .http files into Request structs, handling template syntax and script blocks
- **runner/runner.go**: Manages concurrent execution using goroutines and sync.WaitGroup, with template rendering and script execution
- **reporting/**: Handles multiple output formats and streaming results
- **template/template.go**: Provides template rendering using Go's text/template and JavaScript execution using Goja

Each goroutine executes all requests in the file for the specified number of iterations, with the configured delay between iterations. Before each request execution, templates are rendered using global variables and environment variables. After each request, any associated JavaScript scripts are executed with access to the response data.

## Dependencies

- **github.com/dop251/goja**: JavaScript engine for script execution
- Go standard library for template rendering and HTTP requests
