package reporting

import "time"

// RequestResult represents the outcome of a single HTTP request
type RequestResult struct {
	Name         string
	Verb         string
	URL          string
	StatusCode   int
	ResponseTime time.Duration
	Success      bool
	Error        string
	Timestamp    time.Time
}

// Report contains aggregated statistics from all request executions
type Report struct {
	TotalRequests         int
	SuccessfulRequests    int
	FailedRequests        int
	AverageResponseTime   time.Duration
	MinResponseTime       time.Duration
	MaxResponseTime       time.Duration
	ResponseTimeDistribution map[string]int
	ErrorBreakdown        map[string]int
	RequestDetails        []RequestResult
	StartTime             time.Time
	EndTime               time.Time
}

// ReportFormat defines the output format for reports
type ReportFormat string

const (
	FormatConsole ReportFormat = "console"
	FormatHTML    ReportFormat = "html"
	FormatCSV     ReportFormat = "csv"
	FormatJSON    ReportFormat = "json"
)