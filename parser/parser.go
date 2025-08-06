package parser

import (
	"bufio"
	"curlrunner/http"
	"io/ioutil"
	"strings"
)

// Parse parses a .http file and returns a slice of requests
func Parse(filePath string) ([]http.Request, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	requestsStr := strings.Split(string(content), "###")
	var requests []http.Request

	for _, reqStr := range requestsStr {
		if strings.TrimSpace(reqStr) == "" {
			continue
		}
		requests = append(requests, parseRequest(reqStr))
	}

	return requests, nil
}

func parseRequest(reqStr string) http.Request {
	var request http.Request
	request.Headers = make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(reqStr))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Trim leading empty lines
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}

	if len(lines) == 0 {
		return request
	}

	// Parse verb and URL
	parts := strings.SplitN(lines[0], " ", 2)
	if len(parts) == 2 {
		request.Verb = parts[0]
		request.URL = parts[1]
	}

	// Parse headers and body
	inBody := false
	var bodyLines []string
	for _, line := range lines[1:] {
		if inBody {
			bodyLines = append(bodyLines, line)
			continue
		}
		if line == "" {
			inBody = true
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			request.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	request.Body = strings.Join(bodyLines, "\n")

	return request
}
