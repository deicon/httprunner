package http

// Request represents a single HTTP request
type Request struct {
	Verb    string
	URL     string
	Headers map[string]string
	Body    string
}
