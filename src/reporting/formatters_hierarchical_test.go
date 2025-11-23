package reporting

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/deicon/httprunner/src/reporting/formatters/hierarchical"
	"github.com/deicon/httprunner/src/reporting/types"
)

func sampleHierarchical() *types.HierarchicalReport {
	now := time.Now()
	iter0 := types.IterationReport{
		IterationID:         0,
		TotalRequests:       2,
		SuccessfulRequests:  1,
		FailedRequests:      1,
		AverageResponseTime: 150 * time.Millisecond,
		TotalDuration:       300 * time.Millisecond,
		StartTime:           now,
		EndTime:             now.Add(300 * time.Millisecond),
		RequestResults: []types.RequestResult{
			{Name: "A", Verb: "GET", URL: "/a", StatusCode: 200, Success: true, ResponseTime: 100 * time.Millisecond, Timestamp: now},
			{Name: "B", Verb: "POST", URL: "/b", StatusCode: 500, Success: false, Error: "HTTP 500 Internal Server Error", ResponseTime: 200 * time.Millisecond, Timestamp: now.Add(100 * time.Millisecond), Checks: []types.CheckResult{{Name: "created", Success: false, FailureMessage: "bad"}}},
		},
	}
	iter1 := types.IterationReport{
		IterationID:         1,
		TotalRequests:       1,
		SuccessfulRequests:  1,
		FailedRequests:      0,
		AverageResponseTime: 50 * time.Millisecond,
		TotalDuration:       50 * time.Millisecond,
		StartTime:           now.Add(time.Second),
		EndTime:             now.Add(time.Second + 50*time.Millisecond),
		RequestResults: []types.RequestResult{
			{Name: "C", Verb: "GET", URL: "/c", StatusCode: 200, Success: true, ResponseTime: 50 * time.Millisecond, Timestamp: now.Add(time.Second)},
		},
	}

	vu0 := types.GoroutineReport{
		GoroutineID:          0,
		Iterations:           []types.IterationReport{iter0, iter1},
		TotalIterations:      2,
		SuccessfulIterations: 1,
		FailedIterations:     1,
		TotalRequests:        3,
		SuccessfulRequests:   2,
		FailedRequests:       1,
		AverageResponseTime:  100 * time.Millisecond,
		TotalDuration:        time.Second,
		StartTime:            now,
		EndTime:              now.Add(time.Second),
	}

	summary := types.Report{
		TotalRequests:       3,
		SuccessfulRequests:  2,
		FailedRequests:      1,
		AverageResponseTime: 100 * time.Millisecond,
		StartTime:           now,
		EndTime:             now.Add(2 * time.Second),
		CheckSummaries: map[string]types.CheckSummary{
			"created": {Name: "created", TotalRuns: 1, FailedRuns: 1, FailureMessages: []string{"bad"}},
		},
		TotalChecks:      1,
		SuccessfulChecks: 0,
		FailedChecks:     1,
	}

	return &types.HierarchicalReport{
		Summary:                summary,
		VirtualUserReports:     []types.GoroutineReport{vu0},
		TotalVirtualUsers:      1,
		SuccessfulVirtualUsers: 1,
		FailedVirtualUsers:     0,
	}
}

func TestHierarchicalConsoleSnapshots(t *testing.T) {
	hr := sampleHierarchical()
	f := &hierarchical.HierarchicalFormatter{DetailLevel: types.DetailIteration, Format: types.FormatConsole}
	out, err := f.FormatHierarchical(hr)
	if err != nil {
		t.Fatalf("format console: %v", err)
	}
	for _, s := range []string{
		"HTTP Request Report - Summary",
		"Goroutine Breakdown",
		"Iteration Breakdown",
		"Goroutine 0:",
		"Iteration 0:",
	} {
		if !strings.Contains(out, s) {
			t.Fatalf("console output missing %q\n---\n%s", s, out)
		}
	}
}

func TestHierarchicalJSONSnapshot(t *testing.T) {
	hr := sampleHierarchical()
	f := &hierarchical.HierarchicalFormatter{DetailLevel: types.DetailIteration, Format: types.FormatJSON}
	out, err := f.FormatHierarchical(hr)
	if err != nil {
		t.Fatalf("format json: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	// keys exist
	if _, ok := parsed["summary"]; !ok {
		t.Fatalf("missing summary in json")
	}
	gs, ok := parsed["goroutines"].([]any)
	if !ok || len(gs) != 1 {
		t.Fatalf("goroutines missing or wrong length: %T %v", parsed["goroutines"], parsed["goroutines"])
	}
	g0 := gs[0].(map[string]any)
	if g0["totalIterations"].(float64) != 2 {
		t.Fatalf("expected totalIterations 2, got %v", g0["totalIterations"])
	}
	its, ok := g0["iterations"].([]any)
	if !ok || len(its) != 2 {
		t.Fatalf("iterations missing or wrong length")
	}
}

func TestHierarchicalHTMLSnapshot(t *testing.T) {
	hr := sampleHierarchical()
	f := &hierarchical.HierarchicalFormatter{DetailLevel: types.DetailIteration, Format: types.FormatHTML}
	out, err := f.FormatHierarchical(hr)
	if err != nil {
		t.Fatalf("format html: %v", err)
	}
	for _, s := range []string{
		"<!DOCTYPE html>",
		"HTTP Request Report - Hierarchical",
		"Overall Summary",
		"Total Requests",
	} {
		if !strings.Contains(out, s) {
			t.Fatalf("html output missing %q\n---\n%s", s, out)
		}
	}
}
