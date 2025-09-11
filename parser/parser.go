package parser

import (
	"bufio"
	"github.com/deicon/httprunner/http"

	"os"
	"strings"
)

// Parse parses a .http file and returns a slice of requests
func Parse(filePath string) ([]http.Request, error) {
	content, err := os.ReadFile(filePath)
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

	// Skip any comment lines and extract name and lifecycle annotations if present
	lineIndex := 0
	for lineIndex < len(lines) {
		line := strings.TrimSpace(lines[lineIndex])
		if strings.HasPrefix(line, "# @name ") {
			request.Name = strings.TrimSpace(strings.TrimPrefix(line, "# @name "))
			lineIndex++
		} else if strings.HasPrefix(line, "# @BeforeUser") {
			request.Lifecycle = http.LifecycleBeforeUser
			lineIndex++
		} else if strings.HasPrefix(line, "# @BeforeIteration") {
			request.Lifecycle = http.LifecycleBeforeIteration
			lineIndex++
		} else if strings.HasPrefix(line, "# @TeardownUser") {
			request.Lifecycle = http.LifecycleTeardownUser
			lineIndex++
		} else if strings.HasPrefix(line, "# @TeardownIteration") {
			request.Lifecycle = http.LifecycleTeardownIteration
			lineIndex++
		} else if strings.HasPrefix(line, "#") {
			// Skip regular comment lines
			lineIndex++
		} else if line == "" {
			// Skip empty lines
			lineIndex++
		} else {
			// Found non-comment, non-empty line
			break
		}
	}

	if lineIndex >= len(lines) {
		return request
	}

	// Check for pre-request script block before HTTP verb/URL
	var preScriptLines []string
	if lineIndex < len(lines) && strings.TrimSpace(lines[lineIndex]) == "> {%" {
		lineIndex++ // Skip the opening tag
		inPreScript := true
		for lineIndex < len(lines) && inPreScript {
			line := lines[lineIndex]
			if strings.TrimSpace(line) == "%}" {
				inPreScript = false
			} else {
				preScriptLines = append(preScriptLines, line)
			}
			lineIndex++
		}
		// Skip empty line after script block
		for lineIndex < len(lines) && strings.TrimSpace(lines[lineIndex]) == "" {
			lineIndex++
		}
	}

	request.PreScript = strings.Join(preScriptLines, "\n")

	if lineIndex >= len(lines) {
		return request
	}

	// Skip any additional comment lines before HTTP verb/URL
	for lineIndex < len(lines) {
		line := strings.TrimSpace(lines[lineIndex])
		if strings.HasPrefix(line, "#") {
			lineIndex++
		} else if line == "" {
			lineIndex++
		} else {
			break
		}
	}

	if lineIndex >= len(lines) {
		return request
	}

	// Parse verb and URL (only if not a script-only request)
	line := strings.TrimSpace(lines[lineIndex])
	if line != "> {%" && line != "" && !strings.HasPrefix(line, "#") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			request.Verb = parts[0]
			request.URL = parts[1]
		}
		lineIndex++
	}

	// Parse headers and body
	inBody := false
	inScript := false
	var bodyLines []string
	var scriptLines []string

	for lineIndex < len(lines) {
		line := lines[lineIndex]
		lineIndex++

		// Check for script block start
		if strings.TrimSpace(line) == "> {%" {
			inScript = true
			inBody = false
			continue
		}

		// Check for script block end
		if strings.TrimSpace(line) == "%}" && inScript {
			inScript = false
			continue
		}

		// Handle script content
		if inScript {
			scriptLines = append(scriptLines, line)
			continue
		}

		// Handle body content
		if inBody {
			// Don't add the line if we're starting a script block
			if strings.TrimSpace(line) != "> {%" {
				bodyLines = append(bodyLines, line)
			} else {
				// We found a script block, back up and handle it
				lineIndex--
			}
			continue
		}

		// Empty line marks start of body
		if line == "" {
			inBody = true
			continue
		}

		// Parse headers
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			request.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Join body lines based on Content-Type
	contentType := request.Headers["Content-Type"]
	if contentType == "application/x-www-form-urlencoded" {
		// For form URL encoded, join with no separator and trim spaces/line breaks
		bodyParts := make([]string, 0, len(bodyLines))
		for _, line := range bodyLines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				bodyParts = append(bodyParts, trimmed)
			}
		}
		request.Body = strings.Join(bodyParts, "")
	} else {
		// For other content types (JSON, etc.), preserve line breaks
		request.Body = strings.Join(bodyLines, "\n")
	}

	request.Script = strings.Join(scriptLines, "\n")

	return request
}
