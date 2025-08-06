# Overview
This is a Command line tool, written in Golang to run 
multiple, parallel http requests. 

The tool will be called with 

```bash
./toolname -t 20 -i 10 -d 1000 -f requests.http
```
where -t defines the number of parallel go routines being started. 
Each go routine will use the provided *.http file provided on the -f flag. 

Parameters are as follows: 
-t n   n is the number of parallel go routines
-i n   n is the number of iterations
-d ns  n is the number of milliseconds to wait between iterations
-f filename .http file containing http requests to be executed. 

# Application structure

The main go method will launch -t n go routines. Each routine will execute all requests within the .http files. 
On startup the file needs to be parsed into an array of 

VERB string
url string
header map[string]string
body string

Each go routine will listen on a channel to receive the map of requests as well as the number of iterations and delay between requests as one struct. 

The Goroutine iterates over all requests, calls them, waits the amount of delay ms between each request and terminates. 


# http request file format

1. Each Request is separated by ###
2. the next line contains the HTTP Verb followed by the url to call
3. all following, non empty lines are HTTP Headers. 
4. a blank line
5. either an empty line for no body, or all lines that follow will be a valid json body until an empty line

```txt
### 
POST localhost:8080/api/v3/tarifrechner
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

```
