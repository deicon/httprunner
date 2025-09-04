package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return strings.Join(*m, ", ")
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

type HAR struct {
	Log Log `json:"log"`
}

type Log struct {
	Entries []Entry `json:"entries"`
}

type Entry struct {
	Request Request `json:"request"`
}

type Request struct {
	Method      string      `json:"method"`
	URL         string      `json:"url"`
	Headers     []Header    `json:"headers"`
	PostData    *PostData   `json:"postData,omitempty"`
	QueryString []Parameter `json:"queryString"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type Parameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Config struct {
	InputFile  string
	OutputFile string
	Filters    []string
	Help       bool
}

func main() {
	config := parseFlags()

	if config.Help {
		printHelp()
		return
	}

	if config.InputFile == "" {
		log.Fatal("Input HAR file is required. Use -f flag to specify the file.")
	}

	harData, err := parseHARFile(config.InputFile)
	if err != nil {
		log.Fatalf("Error parsing HAR file: %v", err)
	}

	filteredEntries := filterEntries(harData.Log.Entries, config.Filters)

	if len(filteredEntries) == 0 {
		if len(config.Filters) > 0 {
			fmt.Printf("No requests found matching filters: %s\n", strings.Join(config.Filters, ", "))
		} else {
			fmt.Println("No requests found")
		}
		return
	}

	httpContent := generateHTTPFile(filteredEntries)

	if config.OutputFile == "" {
		fmt.Print(httpContent)
	} else {
		err := writeHTTPFile(config.OutputFile, httpContent)
		if err != nil {
			log.Fatalf("Error writing output file: %v", err)
		}
		fmt.Printf("Successfully extracted %d requests to %s\n", len(filteredEntries), config.OutputFile)
	}
}

func parseFlags() Config {
	var config Config
	var filters multiStringFlag

	flag.StringVar(&config.InputFile, "f", "", "Input HAR file path (required)")
	flag.StringVar(&config.InputFile, "file", "", "Input HAR file path (required)")
	flag.StringVar(&config.OutputFile, "o", "", "Output .http file path (optional, prints to stdout if not specified)")
	flag.StringVar(&config.OutputFile, "output", "", "Output .http file path (optional, prints to stdout if not specified)")
	flag.Var(&filters, "filter", "Filter requests by URL substring (can be used multiple times)")
	flag.BoolVar(&config.Help, "h", false, "Show help")
	flag.BoolVar(&config.Help, "help", false, "Show help")

	flag.Parse()

	config.Filters = []string(filters)
	return config
}

func printHelp() {
	fmt.Println("HAR Parser - Extract HTTP requests from HAR files and convert to .http format")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  harparser -f <har_file> [options]")
	fmt.Println("")
	fmt.Println("Flags:")
	fmt.Println("  -f, -file     Input HAR file path (required)")
	fmt.Println("  -o, -output   Output .http file path (optional, prints to stdout if not specified)")
	fmt.Println("  -filter       Filter requests by URL substring (can be used multiple times)")
	fmt.Println("  -h, -help     Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  harparser -f recording.har -filter kontoeroeffnung -o requests.http")
	fmt.Println("  harparser -f recording.har -filter api/v3")
	fmt.Println("  harparser -f recording.har -filter kontoeroeffnung -filter api/v3 -o requests.http")
	fmt.Println("  harparser -f recording.har -o all_requests.http")
}

func parseHARFile(filename string) (*HAR, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var har HAR
	err = json.Unmarshal(data, &har)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &har, nil
}

func filterEntries(entries []Entry, filters []string) []Entry {
	if len(filters) == 0 {
		return entries
	}

	var filtered []Entry
	for _, entry := range entries {
		// Check if URL matches any of the filters
		matches := false
		for _, filter := range filters {
			if strings.Contains(entry.Request.URL, filter) {
				matches = true
				break
			}
		}
		if matches {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

func generateHTTPFile(entries []Entry) string {
	var content strings.Builder

	for _, entry := range entries {
		req := entry.Request

		content.WriteString("###\n")

		// Add request name based on URL path
		name := extractRequestName(req.URL, req.Method)
		content.WriteString(fmt.Sprintf("# @name %s\n", name))

		// Add HTTP method and URL
		content.WriteString(fmt.Sprintf("%s %s\n", req.Method, req.URL))

		// Add headers (filter out unnecessary ones)
		filteredHeaders := filterHeaders(req.Headers)
		for _, header := range filteredHeaders {
			content.WriteString(fmt.Sprintf("%s: %s\n", header.Name, header.Value))
		}
		content.WriteString("Authorization: Bearer {{.token}}\n")

		// Add request body if present
		if req.PostData != nil && req.PostData.Text != "" {
			content.WriteString("\n")
			content.WriteString(req.PostData.Text)
		}

		// Add separation between requests
		content.WriteString("\n\n")
	}

	return content.String()
}

func extractRequestName(url, method string) string {
	// Remove query parameters
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// Extract path from URL
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		// Use last few meaningful parts of the path
		var nameParts []string
		for i := len(parts) - 1; i >= 0 && len(nameParts) < 3; i-- {
			part := parts[i]
			if part != "" && !isUUID(part) && !isNumeric(part) {
				nameParts = append([]string{part}, nameParts...)
			}
		}

		if len(nameParts) > 0 {
			return fmt.Sprintf("%s %s", method, strings.Join(nameParts, "/"))
		}
	}

	return fmt.Sprintf("%s Request", method)
}

func isUUID(s string) bool {
	return len(s) == 36 && strings.Count(s, "-") == 4
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func filterHeaders(headers []Header) []Header {
	skipHeaders := map[string]bool{
		"cookie":                    true,
		"user-agent":                true,
		"accept-encoding":           true,
		"accept-language":           true,
		"cache-control":             true,
		"pragma":                    true,
		"connection":                true,
		"upgrade-insecure-requests": true,
		"sec-fetch-dest":            true,
		"sec-fetch-mode":            true,
		"sec-fetch-site":            true,
		"sec-fetch-user":            true,
		"sec-ch-ua":                 true,
		"sec-ch-ua-mobile":          true,
		"sec-ch-ua-platform":        true,
		"dnt":                       true,
		"referer":                   true,
		"origin":                    true,
		"request-id":                true,
		"traceparent":               true,
		"sec-fetch-storage-access":  true,
	}

	var filtered []Header
	for _, header := range headers {
		lowerName := strings.ToLower(header.Name)
		if !skipHeaders[lowerName] && !strings.HasPrefix(lowerName, ":") {
			filtered = append(filtered, header)
		}
	}

	return filtered
}

func writeHTTPFile(filename, content string) error {
	return os.WriteFile(filename, []byte(content), 0644)
}
