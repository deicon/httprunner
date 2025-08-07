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
./httprunner -t <threads> -i <iterations> -d <delay> -f <file>
```

### Parameters
- `-t n`: Number of parallel goroutines (default: 1)
- `-i n`: Number of iterations (default: 1)  
- `-d n`: Delay between iterations in milliseconds (default: 0)
- `-f filename`: .http file containing HTTP requests (required)

### Example
```bash
./httprunner -t 20 -i 10 -d 1000 -f requests.http
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

Post-request JavaScript code can be embedded using `> {%` and `%}` blocks. The script has access to:
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
POST {{.BASEURL}}/api/{{.APIVERSION}}/tarifrechner
Authorization: Bearer {{.TOKEN}}
Content-Type: application/json

{
"tarifrechnerModus": {
"modus": "{{.MODUS}}",
"mandant": "ORGA",
"haushaltsId": "{{.HAUSHALTSID}}"
},
"produktKonfigurationId": "Kontoeroeffnung",
"kundennummern": [
    "99152160"
]
}

> {%
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

# Run with sample requests
./httprunner -t 1 -i 1 -d 0 -f requests.http
```

## Architecture

- **main.go**: Handles CLI flag parsing and coordinates the parser and runner
- **http/requests.go**: Defines the Request struct containing Name, Verb, URL, Headers, Body, and Script
- **parser/parser.go**: Parses .http files into Request structs, handling template syntax and script blocks
- **runner/runner.go**: Manages concurrent execution using goroutines and sync.WaitGroup, with template rendering and script execution
- **template/template.go**: Provides template rendering using Go's text/template and JavaScript execution using Goja

Each goroutine executes all requests in the file for the specified number of iterations, with the configured delay between iterations. Before each request execution, templates are rendered using global variables and environment variables. After each request, any associated JavaScript scripts are executed with access to the response data.

## Dependencies

- **github.com/dop251/goja**: JavaScript engine for script execution
- Go standard library for template rendering and HTTP requests