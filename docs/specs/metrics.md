# Metrics
This document describes the metrics collected by the system for monitoring and analysis purposes.
## Available Metrics
### Standard built-in metrics

|Metric Name|	Type|	Description|
|___________|_______|_______________|
|checks|	Rate|	The rate of successful checks.|
|data_received|	Counter|	The amount of received data. This example covers how to track data for an individual URL.|
|data_sent|	Counter|	The amount of data sent. Track data for an individual URL to track data for an individual URL.|
|dropped_iterations|	Counter|	The number of iterations that weren’t started due to lack of VUs (for the arrival-rate executors) or lack of time (expired maxDuration in the iteration-based executors). Refer to Dropped iterations for more details.|
|iteration_duration|	Trend|	The time to complete one full iteration, including time spent in setup and teardown. To calculate the duration of the iteration’s function for the specific scenario, try this workaround.|
|iterations|	Counter|	The aggregate number of times the VUs execute the JS script (the default function).|
|vus|	Gauge|	Current number of active virtual users|
|vus_max|	Gauge|	Max possible number of virtual users (VU resources are pre-allocated, to avoid affecting performance when scaling up load).|

### HTTP metrics
|Metric Name|	Type|	Description|
|___________|_______|_______________|
|http_req_blocked|	Trend|	Time spent blocked before a request (e.g. waiting for a free socket).|
|http_req_connecting|	Trend|	Time spent establishing a TCP connection.|
|http_req_duration|	Trend|	Time spent from the start of the request until the response body is fully received.|
|http_req_failed|	Rate|	The rate of failed HTTP requests.|
|http_req_receiving|	Trend|	Time spent receiving the response data.|
|http_req_sending|	Trend|	Time spent sending the request data.|
|http_req_tls_handshaking|	Trend|	Time spent performing TLS handshake.|
|http_req_waiting|	Trend|	Time spent waiting for a response from the server (a.k.a. time to first byte).|
|http_reqs|	Counter|	The number of HTTP requests made.|
