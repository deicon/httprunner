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
	GoroutineID  int
	IterationID  int
}

// Report contains aggregated statistics from all request executions
type Report struct {
	TotalRequests            int
	SuccessfulRequests       int
	FailedRequests           int
	AverageResponseTime      time.Duration
	MinResponseTime          time.Duration
	MaxResponseTime          time.Duration
	ResponseTimeDistribution map[string]int
	ErrorBreakdown           map[string]int
	RequestDetails           []RequestResult
	StartTime                time.Time
	EndTime                  time.Time
}

// IterationReport contains results for a single iteration
type IterationReport struct {
	IterationID         int
	RequestResults      []RequestResult
	TotalRequests       int
	SuccessfulRequests  int
	FailedRequests      int
	AverageResponseTime time.Duration
	TotalDuration       time.Duration
	StartTime           time.Time
	EndTime             time.Time
}

// GoroutineReport contains results for a single goroutine
type GoroutineReport struct {
	GoroutineID          int
	Iterations           []IterationReport
	TotalIterations      int
	SuccessfulIterations int
	FailedIterations     int
	TotalRequests        int
	SuccessfulRequests   int
	FailedRequests       int
	AverageResponseTime  time.Duration
	TotalDuration        time.Duration
	StartTime            time.Time
	EndTime              time.Time
}

// HierarchicalReport provides detailed breakdown by goroutines and iterations
type HierarchicalReport struct {
	Summary              Report
	Goroutines           []GoroutineReport
	TotalGoroutines      int
	SuccessfulGoroutines int
	FailedGoroutines     int
}

// ReportDetailLevel controls how much detail to show in reports
type ReportDetailLevel string

const (
	DetailSummary   ReportDetailLevel = "summary"   // Only overall summary
	DetailGoroutine ReportDetailLevel = "goroutine" // Summary + goroutine breakdown
	DetailIteration ReportDetailLevel = "iteration" // Summary + goroutine + iteration breakdown
	DetailFull      ReportDetailLevel = "full"      // All levels + individual requests
)

// ReportFormat defines the output format for reports
type ReportFormat string

const (
	FormatConsole ReportFormat = "console"
	FormatHTML    ReportFormat = "html"
	FormatCSV     ReportFormat = "csv"
	FormatJSON    ReportFormat = "json"
)
