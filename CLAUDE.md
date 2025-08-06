# curlrunner

A Go-based command-line tool for running multiple, parallel HTTP requests.

## Project Structure

```
curlrunner/
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
./curlrunner -t <threads> -i <iterations> -d <delay> -f <file>
```

### Parameters
- `-t n`: Number of parallel goroutines (default: 1)
- `-i n`: Number of iterations (default: 1)  
- `-d n`: Delay between iterations in milliseconds (default: 0)
- `-f filename`: .http file containing HTTP requests (required)

### Example
```bash
./curlrunner -t 20 -i 10 -d 1000 -f requests.http
```

## HTTP Request File Format

Requests are separated by `###` and follow this format:

```
### 
<HTTP_VERB> <URL>
<Header-Name>: <Header-Value>
<Header-Name>: <Header-Value>

<JSON_BODY>

### 
<HTTP_VERB> <URL>

```

### Example requests.http
```
### 
POST http://localhost:8080/api/v3/tarifrechner
Content-Type: application/json

{
  "tarifrechnerModus": {
    "modus": "TARIFRECHNER",
    "mandant": "DVAG",
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

## Building and Testing

```bash
# Build the project
go build

# Run tests
go test ./parser

# Run with sample requests
./curlrunner -t 1 -i 1 -d 0 -f requests.http
```

## Architecture

- **main.go**: Handles CLI flag parsing and coordinates the parser and runner
- **http/requests.go**: Defines the Request struct containing Verb, URL, Headers, and Body
- **parser/parser.go**: Parses .http files into Request structs
- **runner/runner.go**: Manages concurrent execution using goroutines and sync.WaitGroup

Each goroutine executes all requests in the file for the specified number of iterations, with the configured delay between iterations.