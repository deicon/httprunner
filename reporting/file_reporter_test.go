package reporting

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deicon/httprunner/reporting/types"
)

// writeJSONL writes RequestResult lines to a temp file and returns the path
func writeJSONL(t *testing.T, dir, name string, results []types.RequestResult) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	w := bufio.NewWriter(f)
	for _, r := range results {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	_ = f.Close()
	return p
}

func TestFileReporter_GenerateReport_Summary(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	// Build sample results: 5 total, 3 success, 2 fail
	results := []types.RequestResult{
		{Name: "vu0-i0-a", Verb: "GET", URL: "/a", StatusCode: 200, ResponseTime: 50 * time.Millisecond, Success: true, Timestamp: base.Add(10 * time.Millisecond), VirtualUserID: 0, IterationID: 0, Checks: []types.CheckResult{{Name: "ok", Success: true}}},
		{Name: "vu0-i0-b", Verb: "POST", URL: "/b", StatusCode: 500, ResponseTime: 600 * time.Millisecond, Success: false, Error: "HTTP 500 Internal Server Error", Timestamp: base.Add(20 * time.Millisecond), VirtualUserID: 0, IterationID: 0, Checks: []types.CheckResult{{Name: "created", Success: false, FailureMessage: "not ok"}}},
		{Name: "vu0-i1-a", Verb: "GET", URL: "/a1", StatusCode: 200, ResponseTime: 120 * time.Millisecond, Success: true, Timestamp: base.Add(110 * time.Millisecond), VirtualUserID: 0, IterationID: 1, Checks: []types.CheckResult{{Name: "ok", Success: true}}},
		{Name: "vu1-i0-a", Verb: "GET", URL: "/c", StatusCode: 200, ResponseTime: 300 * time.Millisecond, Success: true, Timestamp: base.Add(210 * time.Millisecond), VirtualUserID: 1, IterationID: 0, Checks: []types.CheckResult{{Name: "ok", Success: true}}},
		{Name: "vu1-i1-a", Verb: "POST", URL: "/d", StatusCode: 500, ResponseTime: 1100 * time.Millisecond, Success: false, Error: "HTTP 500 Internal Server Error", Timestamp: base.Add(310 * time.Millisecond), VirtualUserID: 1, IterationID: 1, Checks: []types.CheckResult{{Name: "created", Success: false, FailureMessage: "not ok"}}},
	}
	file := writeJSONL(t, dir, "results.jsonl", results)

	fr := NewFileReporter(file)
	rep, err := fr.GenerateReport(base)
	if err != nil {
		t.Fatalf("GenerateReport: %v", err)
	}

	if rep.TotalRequests != 5 || rep.SuccessfulRequests != 3 || rep.FailedRequests != 2 {
		t.Fatalf("totals mismatch: %+v", *rep)
	}
	if rep.MinResponseTime != 50*time.Millisecond || rep.MaxResponseTime != 1100*time.Millisecond {
		t.Fatalf("min/max mismatch: min=%v max=%v", rep.MinResponseTime, rep.MaxResponseTime)
	}
	if rep.ResponseTimeDistribution["<100ms"] != 1 || rep.ResponseTimeDistribution["100-500ms"] != 2 || rep.ResponseTimeDistribution["500ms-1s"] != 1 || rep.ResponseTimeDistribution[">1s"] != 1 {
		t.Fatalf("distribution mismatch: %#v", rep.ResponseTimeDistribution)
	}
	if rep.ErrorBreakdown["HTTP 500 Internal Server Error"] != 2 {
		t.Fatalf("error breakdown mismatch: %#v", rep.ErrorBreakdown)
	}
	// Check summaries aggregated with de-dup of failure messages
	created := rep.CheckSummaries["created"]
	if created.TotalRuns != 2 || created.FailedRuns != 2 || len(created.FailureMessages) != 1 || created.FailureMessages[0] != "not ok" {
		t.Fatalf("check summary mismatch: %#v", created)
	}
	if len(rep.RequestDetails) != len(results) || rep.RequestDetails[0].Name != "vu0-i0-a" || rep.RequestDetails[len(results)-1].Name != "vu1-i1-a" {
		t.Fatalf("request details ordering mismatch")
	}
	// Average equals sum/5 (50+600+120+300+1100 = 2170ms)
	if rep.AverageResponseTime != (2170*time.Millisecond)/5 {
		t.Fatalf("average mismatch: %v", rep.AverageResponseTime)
	}
}

func TestFileReporter_GenerateHierarchicalReport(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	results := []types.RequestResult{
		{Name: "vu0-i0-a", Verb: "GET", URL: "/a", StatusCode: 200, ResponseTime: 50 * time.Millisecond, Success: true, Timestamp: base.Add(10 * time.Millisecond), VirtualUserID: 0, IterationID: 0},
		{Name: "vu0-i0-b", Verb: "POST", URL: "/b", StatusCode: 500, ResponseTime: 600 * time.Millisecond, Success: false, Error: "HTTP 500 Internal Server Error", Timestamp: base.Add(20 * time.Millisecond), VirtualUserID: 0, IterationID: 0},
		{Name: "vu0-i1-a", Verb: "GET", URL: "/a1", StatusCode: 200, ResponseTime: 120 * time.Millisecond, Success: true, Timestamp: base.Add(110 * time.Millisecond), VirtualUserID: 0, IterationID: 1},
		{Name: "vu1-i0-a", Verb: "GET", URL: "/c", StatusCode: 200, ResponseTime: 300 * time.Millisecond, Success: true, Timestamp: base.Add(210 * time.Millisecond), VirtualUserID: 1, IterationID: 0},
		{Name: "vu1-i1-a", Verb: "POST", URL: "/d", StatusCode: 500, ResponseTime: 1100 * time.Millisecond, Success: false, Error: "HTTP 500 Internal Server Error", Timestamp: base.Add(310 * time.Millisecond), VirtualUserID: 1, IterationID: 1},
	}
	file := writeJSONL(t, dir, "results.jsonl", results)

	fr := NewFileReporter(file)
	hr, err := fr.GenerateHierarchicalReport(base)
	if err != nil {
		t.Fatalf("GenerateHierarchicalReport: %v", err)
	}

	if hr.TotalVirtualUsers != 2 {
		t.Fatalf("TotalVirtualUsers mismatch: %d", hr.TotalVirtualUsers)
	}
	if len(hr.VirtualUserReports) != 2 {
		t.Fatalf("unexpected VU reports count: %d", len(hr.VirtualUserReports))
	}

	// Find reports by GoroutineID
	var vu0, vu1 *types.GoroutineReport
	for i := range hr.VirtualUserReports {
		if hr.VirtualUserReports[i].GoroutineID == 0 {
			vu0 = &hr.VirtualUserReports[i]
		}
		if hr.VirtualUserReports[i].GoroutineID == 1 {
			vu1 = &hr.VirtualUserReports[i]
		}
	}
	if vu0 == nil || vu1 == nil {
		t.Fatalf("missing vu0 or vu1 report")
	}

	if vu0.TotalIterations != 2 || vu1.TotalIterations != 2 {
		t.Fatalf("iterations mismatch: vu0=%d vu1=%d", vu0.TotalIterations, vu1.TotalIterations)
	}
	// vu0 has one failed iteration (iteration 0) and one successful (iteration 1)
	if vu0.SuccessfulIterations != 1 || vu0.FailedIterations != 1 {
		t.Fatalf("vu0 iteration success/fail mismatch: %+v", *vu0)
	}
	// vu1 has one successful (iter 0) and one failed (iter 1)
	if vu1.SuccessfulIterations != 1 || vu1.FailedIterations != 1 {
		t.Fatalf("vu1 iteration success/fail mismatch: %+v", *vu1)
	}
	// Check per-iteration request counts and average for vu0 iter0
	var vu0i0 *types.IterationReport
	for i := range vu0.Iterations {
		if vu0.Iterations[i].IterationID == 0 {
			vu0i0 = &vu0.Iterations[i]
		}
	}
	if vu0i0 == nil {
		t.Fatalf("missing vu0 iter0")
	}
	if vu0i0.TotalRequests != 2 || vu0i0.AverageResponseTime != (50*time.Millisecond+600*time.Millisecond)/2 {
		t.Fatalf("vu0 iter0 totals/avg mismatch: %+v", *vu0i0)
	}
	if len(vu0i0.RequestResults) != 2 || vu0i0.RequestResults[0].Name != "vu0-i0-a" {
		t.Fatalf("vu0 iter0 results mismatch: %+v", vu0i0.RequestResults)
	}
}

func TestFileReporter_ErrorPaths(t *testing.T) {
	fr := NewFileReporter(filepath.Join(t.TempDir(), "missing.jsonl"))
	if _, err := fr.GenerateReport(time.Now()); err == nil {
		t.Fatalf("expected error on missing file for GenerateReport")
	}
	if _, err := fr.GenerateHierarchicalReport(time.Now()); err == nil {
		t.Fatalf("expected error on missing file for GenerateHierarchicalReport")
	}

	// Invalid JSON line
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(p, []byte("not json\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fr2 := NewFileReporter(p)
	if _, err := fr2.GenerateReport(time.Now()); err == nil {
		t.Fatalf("expected unmarshal error in GenerateReport")
	}
	if _, _, err := fr2.collectAllDataInSinglePass(); err == nil { // indirectly used by hierarchical
		t.Fatalf("expected unmarshal error in collectAllDataInSinglePass")
	}
}
