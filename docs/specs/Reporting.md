# Overview

When the tool has completed executing all HTTP requests, it will generate a comprehensive report summarizing the
performance and outcomes of the requests made. This report will include key metrics and statistics to help users analyze
the results of their HTTP requests.

# Report Contents

The report will contain the following information:

1. **Total Requests**: The total number of HTTP requests made during the execution.
2. **Successful Requests**: The number of requests that received a successful response (HTTP status codes 200-299).
3. **Failed Requests**: The number of requests that failed (HTTP status codes 400-599).
4. **Average Response Time**: The average time taken to receive a response for all requests.
5. **Minimum Response Time**: The shortest time taken to receive a response.
6. **Maximum Response Time**: The longest time taken to receive a response.
7. **Response Time Distribution**: A breakdown of response times into categories (e.g., <100ms, 100-500ms, 500ms-1s, >
   1s).
8. **Error Breakdown**: A summary of the types of errors encountered (e.g., 404 Not Found, 500 Internal Server Error).
9. **Request Details**: A detailed log of each request made, including:
    - HTTP method
    - URL
    - Status code
    - Response time
    - Any error messages (if applicable)

# Report Visualization

The report can be presented in various formats, including:

- **Console Output**: A simple text-based summary displayed in the terminal.
- **HTML Report**: A more detailed and visually appealing report that can be opened in a web browser.
- **CSV/JSON Export**: For further analysis, the report can be exported in CSV or JSON format.

# Example Console Report

```
HTTP Request Report
-------------------
Total Requests: 1000
Successful Requests: 950
Failed Requests: 50
Average Response Time: 250ms
Minimum Response Time: 50ms
Maximum Response Time: 1200ms
Response Time Distribution:
- <100ms: 300
- 100-500ms: 600
- 500ms-1s: 80
-  >1s: 20
Error Breakdown:
- 404 Not Found: 30
- 500 Internal Server Error: 20
Request Details:
1. GET http://example.com/api/resource - 200 OK - 150ms
2. POST http://example.com/api/resource - 500 Internal Server Error - 1200

...
``` 

# Implementation

To implement the reporting feature, the tool will need to collect data during the execution of HTTP requests
and store it in a structured format. After all requests have been completed, the tool will process this data to generate
the report. This may involve using data structures to track request outcomes and response times, as well as functions to
format and display the report in the desired format.

The Application will use channels to collect results from each goroutine and aggregate them for reporting. A separate reporting module
can be created to handle the formatting and output of the report.
An additional flags can be added to the command line tool to specify the desired report format (e.g., `-report html` or `-report csv` or `-report json` ).



