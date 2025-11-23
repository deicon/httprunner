package reporting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deicon/httprunner/src/metrics"
	"github.com/deicon/httprunner/src/reporting/formatters/console"
	"github.com/deicon/httprunner/src/reporting/formatters/csv"
	"github.com/deicon/httprunner/src/reporting/formatters/html"
	jsonfmt "github.com/deicon/httprunner/src/reporting/formatters/json"
	"github.com/deicon/httprunner/src/reporting/types"
)

func sampleReport() *types.Report {
	now := time.Now()
	// Request details: one success, one failure with checks
	reqs := []types.RequestResult{
		{
			Name:         "GET /api/items",
			Verb:         "GET",
			URL:          "https://example.com/api/items",
			StatusCode:   200,
			ResponseTime: 120 * time.Millisecond,
			Success:      true,
			Timestamp:    now,
			Checks:       []types.CheckResult{{Name: "status ok", Success: true}},
		},
		{
			Name:         "POST /api/items",
			Verb:         "POST",
			URL:          "https://example.com/api/items",
			StatusCode:   500,
			ResponseTime: 800 * time.Millisecond,
			Success:      false,
			Error:        "HTTP 500 Internal Server Error",
			Timestamp:    now.Add(time.Second),
			Checks: []types.CheckResult{
				{Name: "created", Success: false, FailureMessage: "not created"},
			},
		},
	}

	// Check summaries
	checks := map[string]types.CheckSummary{
		"status ok": {Name: "status ok", TotalRuns: 1, SuccessfulRuns: 1, FailedRuns: 0},
		"created":   {Name: "created", TotalRuns: 1, SuccessfulRuns: 0, FailedRuns: 1, FailureMessages: []string{"not created"}},
	}

	// Metrics summaries across types
	metricsSummaries := map[string]metrics.MetricSummary{
		"http_reqs":         {Name: "http_reqs", Type: metrics.Counter, Count: 2, Sum: 2, Rate: 2, RateUnit: "req/s"},
		"http_req_duration": {Name: "http_req_duration", Type: metrics.Trend, Count: 2, Average: 460, Min: 120, Max: 800, P50: 460, P90: 800, P95: 800, P99: 800},
		"http_req_failed":   {Name: "http_req_failed", Type: metrics.Rate, Count: 2, Average: 0.5},
		"vus":               {Name: "vus", Type: metrics.Gauge, Count: 1, Average: 1, LatestValue: 1},
	}

	return &types.Report{
		TotalRequests:            2,
		SuccessfulRequests:       1,
		FailedRequests:           1,
		AverageResponseTime:      460 * time.Millisecond,
		MinResponseTime:          120 * time.Millisecond,
		MaxResponseTime:          800 * time.Millisecond,
		ResponseTimeDistribution: map[string]int{"<100ms": 0, "100-500ms": 1, "500ms-1s": 1, ">1s": 0},
		ErrorBreakdown:           map[string]int{"HTTP 500 Internal Server Error": 1},
		RequestDetails:           reqs,
		StartTime:                now,
		EndTime:                  now.Add(2 * time.Second),
		CheckSummaries:           checks,
		TotalChecks:              2,
		SuccessfulChecks:         1,
		FailedChecks:             1,
		MetricsSummaries:         metricsSummaries,
		TotalVirtualUsers:        1,
		RuntimeSeconds:           2,
		PerVUMetrics:             map[string]float64{"http_reqs_per_vu": 2},
		PerIterationMetrics:      map[string]float64{"data_sent_per_iter": 100},
	}
}

func TestConsoleFormatterSnapshot(t *testing.T) {
	rep := sampleReport()
	f := &console.ConsoleFormatter{}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("console format error: %v", err)
	}
	// Spot-check important sections and values
	checks := []string{
		"HTTP Request Report",
		"Total Requests: 2 (1.0 req/s)",
		"Successful Requests: 1",
		"Failed Requests: 1",
		"Response Time Distribution:",
		"- 100-500ms: 1",
		"- 500ms-1s: 1",
		"Error Breakdown:",
		"HTTP 500 Internal Server Error: 1",
		"Check Results:",
		"status ok: 1 runs (1 successful, 0 failed)",
		"created: 1 runs (0 successful, 1 failed)",
		"All Collected Metrics:",
		"Counters:",
		"Trends:",
		"Rates:",
		"Gauges:",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Fatalf("console output missing %q\n---\n%s", s, out)
		}
	}
}

func TestJSONFormatterSnapshot(t *testing.T) {
	rep := sampleReport()
	f := &jsonfmt.JSONFormatter{}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("json format error: %v", err)
	}
	// Valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	// Spot-check keys in JSON string
	for _, s := range []string{"\"summary\"", "\"totalRequests\": 2", "\"requestDetails\""} {
		if !strings.Contains(out, s) {
			t.Fatalf("json output missing %q\n---\n%s", s, out)
		}
	}
}

func TestCSVFormatterSnapshot(t *testing.T) {
	rep := sampleReport()
	f := &csv.CSVFormatter{}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("csv format error: %v", err)
	}
	// Header and at least two records with expected fields
	if !strings.HasPrefix(out, "Index,Name,Method,URL,Success,StatusCode,ResponseTime,Error,CheckFailures,Timestamp\n") {
		t.Fatalf("csv header mismatch:\n%s", out)
	}
	if !strings.Contains(out, "1,GET /api/items,GET,https://example.com/api/items,true,200,120ms,,") {
		t.Fatalf("csv missing first record fields:\n%s", out)
	}
	if !strings.Contains(out, ",POST /api/items,POST,https://example.com/api/items,false,500,800ms,HTTP 500 Internal Server Error,not created,") {
		t.Fatalf("csv missing second record fields:\n%s", out)
	}
}

func TestHTMLFormatterSnapshot(t *testing.T) {
	rep := sampleReport()
	f := &html.HTMLFormatter{}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("html format error: %v", err)
	}
	// Verify title and some key metrics render
	checks := []string{
		"<title>HTTP Request Report</title>",
		"<strong>Total Requests:</strong> 2",
		"<strong>Successful:</strong>",
		"<strong>Failed:</strong>",
		"Response Time Distribution",
		"Error Breakdown",
		"Check Results",
		"📊 All Collected Metrics",
	}
	for _, s := range checks {
		if !strings.Contains(out, s) {
			t.Fatalf("html output missing %q\n---\n%s", s, out)
		}
	}
}

func TestGetFormatterDispatch(t *testing.T) {
	if _, ok := GetFormatter(types.FormatHTML).(*html.HTMLFormatter); !ok {
		t.Fatalf("GetFormatter HTML wrong type")
	}
	if _, ok := GetFormatter(types.FormatCSV).(*csv.CSVFormatter); !ok {
		t.Fatalf("GetFormatter CSV wrong type")
	}
	if _, ok := GetFormatter(types.FormatJSON).(*jsonfmt.JSONFormatter); !ok {
		t.Fatalf("GetFormatter JSON wrong type")
	}
	if _, ok := GetFormatter("console").(*console.ConsoleFormatter); !ok {
		t.Fatalf("GetFormatter default wrong type")
	}
}
