package http

// Request represents a single HTTP request
type Request struct {
	Name      string
	Verb      string
	URL       string
	Headers   map[string]string
	Body      string
	PreScript string
	Script    string
}
