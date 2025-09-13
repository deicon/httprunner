package streaming

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deicon/httprunner/reporting/types"
)

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Fatalf("close file: %v", err)
		}
	}()
	var lines []string
	s := bufio.NewScanner(f)
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return lines
}

func TestStreamingCollectorWriteAndClose(t *testing.T) {
	dir := t.TempDir()
	sc, err := NewStreamingCollector(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatalf("NewStreamingCollector: %v", err)
	}
	if sc.GetStartTime().IsZero() {
		t.Fatalf("start time not set")
	}

	// Write a few results
	for i := 0; i < 3; i++ {
		r := types.RequestResult{Name: "req", Verb: "GET", URL: "/x", StatusCode: 200, Success: true, ResponseTime: time.Millisecond, Timestamp: time.Now()}
		if err := sc.AddResult(r); err != nil {
			t.Fatalf("AddResult: %v", err)
		}
	}
	if sc.GetResultCount() != 3 {
		t.Fatalf("expected result count 3, got %d", sc.GetResultCount())
	}

	path := sc.GetResultsFilePath()
	if path == "" {
		t.Fatalf("results file path empty")
	}

	if err := sc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify file content
	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	var rr types.RequestResult
	if err := json.Unmarshal([]byte(lines[0]), &rr); err != nil {
		t.Fatalf("line is not JSON: %v", err)
	}
	if rr.Name != "req" || rr.StatusCode != 200 {
		t.Fatalf("unexpected first record: %+v", rr)
	}
}
