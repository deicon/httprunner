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
./httprunner -u <virtualuser> -i <iterations> -d <delay> -f <file>
```

### Parameters
- `-u n`: Number of parallel virtualusers (default: 1)
- `-i n`: Number of iterations (default: 1)  
- `-d n`: Delay between iterations in milliseconds (default: 0)
- `-f filename`: .http file containing HTTP requests (required)

### Example
```bash
./httprunner -u 20 -i 10 -d 1000 -f requests.http
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
   - `client.check(name, checkHandler, failureMessage)`: Perform validation checks (see Check Functions section)

2. **Post-request scripts**: Placed after the request body. These scripts execute after the request is sent and have access to:
   - `response.body`: The response body (parsed as JSON if valid, otherwise as string)
   - `client.global.set(key, value)`: Store values in global variables
   - `client.global.get(key)`: Retrieve values from global variables
   - `client.check(name, checkHandler, failureMessage)`: Perform validation checks (see Check Functions section)

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
    // Note: This is a conceptual example - actual response time would need to be tracked differently
    return true; // Placeholder for response time validation
}, "Response time should be acceptable");
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