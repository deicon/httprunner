# httprunner

A Go-based command-line tool for running multiple, parallel HTTP requests.

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
./httprunner -t <threads> -i <iterations> -d <delay> -f <file> [options]
```

### Parameters
#### Core Parameters
- `-t n`: Number of parallel virtualusers (default: 1)
- `-i n`: Number of iterations (default: 1)  
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
# @name Tarifrechner anlegen
> {%
// Pre-request script: Set up dynamic values before the request
client.global.set("currentTimestamp", Date.now().toString())
client.global.set("requestId", "req_" + Math.random().toString(36).substring(7))
%}

POST {{.BASEURL}}/api/{{.APIVERSION}}/tarifrechner
Authorization: Bearer {{.TOKEN}}
Content-Type: application/json
X-Request-ID: {{.requestId}}

{
"tarifrechnerModus": {
"modus": "{{.MODUS}}",
"mandant": "ORGA",
"haushaltsId": "{{.HAUSHALTSID}}"
},
"produktKonfigurationId": "Kontoeroeffnung",
"kundennummern": [
    "99152160"
],
"timestamp": "{{.currentTimestamp}}"
}

> {%
// Post-request script: Extract values from response
var jsonData = response.body
client.global.set("tarifrechnerId", jsonData.id)
client.global.set("beteiligter_1", jsonData.beteiligte ? jsonData.beteiligte[0].id : "default")
%}

### 
# @name Get Tarifrechner
GET {{.BASEURL}}/api/{{.APIVERSION}}/tarifrechner/{{.tarifrechnerId}}
Authorization: Bearer {{.TOKEN}}
Accept: application/json

###
```

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

The application automatically chooses between traditional in-memory execution and streaming mode based on workload size and user preferences, ensuring optimal performance for both small and large-scale testing scenarios.

## Dependencies

- **github.com/dop251/goja**: JavaScript engine for script execution
- Go standard library for template rendering and HTTP requests