package main

import (
	"flag"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempHTTP writes a minimal .http file to path
func writeTempHTTP(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	_ = w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestRunnerConsoleSummaryWithScriptOnly(t *testing.T) {
	dir := t.TempDir()
	httpPath := writeTempHTTP(t, dir, "script_only.http", `###
# @name Script Only
> {%
client.global.set("x", 1)
%}
`)

	// Isolate flags
	oldFS := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(new(strings.Builder))
	defer func() { flag.CommandLine = oldFS }()

	// Configure args
	oldArgs := os.Args
	os.Args = []string{"httprunner", "-f", httpPath, "-report", "console", "-detail", "summary", "-output", dir, "-u", "1", "-i", "1"}
	defer func() { os.Args = oldArgs }()

	out := captureStdout(func() { main() })

	if !strings.Contains(out, "Raw results available in:") {
		t.Fatalf("expected summary output; got:\n%s", out)
	}
}

func TestRunnerHierarchicalConsole(t *testing.T) {
	dir := t.TempDir()
	httpPath := writeTempHTTP(t, dir, "script_only.http", `###
# @name Script Only
> {%
/* no-op */
%}
`)

	// Isolate flags
	oldFS := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(new(strings.Builder))
	defer func() { flag.CommandLine = oldFS }()

	// Configure args for hierarchical detail
	oldArgs := os.Args
	os.Args = []string{"httprunner", "-f", httpPath, "-report", "console", "-detail", "iteration", "-output", dir, "-u", "1", "-i", "1"}
	defer func() { os.Args = oldArgs }()

	out := captureStdout(func() { main() })
	if !strings.Contains(out, "HTTP Request Report - Summary") {
		t.Fatalf("expected hierarchical console summary; got:\n%s", out)
	}
}

func TestRunnerWritesJSONReportFile(t *testing.T) {
	dir := t.TempDir()
	httpPath := writeTempHTTP(t, dir, "script_only.http", `###
# @name Script Only
> {% %}
`)

	// Isolate flags
	oldFS := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(new(strings.Builder))
	defer func() { flag.CommandLine = oldFS }()

	oldArgs := os.Args
	os.Args = []string{"httprunner", "-f", httpPath, "-report", "json", "-detail", "summary", "-output", dir}
	defer func() { os.Args = oldArgs }()

	out := captureStdout(func() { main() })
	// Expect save message and raw results message
	if !strings.Contains(out, "Formatted report saved to:") || !strings.Contains(out, "Raw results available in:") {
		t.Fatalf("expected file save messages; got:\n%s", out)
	}

	// Verify file exists
	jsonPath := filepath.Join(dir, "report.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("expected %s to exist: %v", jsonPath, err)
	}
}

func TestRunnerExitsOnMissingFileFlag(t *testing.T) {
	cmd := exec.Command("go", "run", ".")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit when -f missing; output: %s", string(out))
	}
	if !strings.Contains(string(out), "Error: -f flag is required") {
		t.Fatalf("missing required flag message; got: %s", string(out))
	}
}

func TestRunnerExitsOnInvalidReportFormat(t *testing.T) {
	dir := t.TempDir()
	httpPath := writeTempHTTP(t, dir, "script_only.http", "###\n# @name X\n> {% %}\n")
	cmd := exec.Command("go", "run", ".", "-f", httpPath, "-report", "bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid report format; output: %s", string(out))
	}
	if !strings.Contains(string(out), "Invalid report format") {
		t.Fatalf("expected invalid format message; got: %s", string(out))
	}
}

func TestRunnerExitsOnInvalidDetailLevel(t *testing.T) {
	dir := t.TempDir()
	httpPath := writeTempHTTP(t, dir, "script_only.http", "###\n# @name X\n> {% %}\n")
	cmd := exec.Command("go", "run", ".", "-f", httpPath, "-detail", "bogus")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid detail; output: %s", string(out))
	}
	if !strings.Contains(string(out), "Invalid report detail level") {
		t.Fatalf("expected invalid detail message; got: %s", string(out))
	}
}

func TestOfflineCSVShortcutPrintsToStdout(t *testing.T) {
    dir := t.TempDir()
    // Write minimal JSONL with a single RequestResult
    jsonl := `{"Name":"Req1","Verb":"GET","URL":"http://example.com","StatusCode":200,"ResponseTime":100000000,"Success":true,"Error":"","Timestamp":"2024-01-01T00:00:00Z","VirtualUserID":1,"IterationID":1,"Checks":[]}`
    p := filepath.Join(dir, "results.jsonl")
    if err := os.WriteFile(p, []byte(jsonl+"\n"), 0644); err != nil {
        t.Fatalf("write jsonl: %v", err)
    }

    // Isolate flags
    oldFS := flag.CommandLine
    flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
    flag.CommandLine.SetOutput(new(strings.Builder))
    defer func() { flag.CommandLine = oldFS }()

    oldArgs := os.Args
    os.Args = []string{"httprunner", "-raw", p, "-csv"}
    defer func() { os.Args = oldArgs }()

    out := captureStdout(func() { main() })
    if !strings.Contains(out, "Index,Name,Method,URL,Success,StatusCode,ResponseTime,Error,CheckFailures,Timestamp") {
        t.Fatalf("expected CSV header in output; got:\n%s", out)
    }
    if !strings.Contains(out, "Req1") {
        t.Fatalf("expected CSV row with request name; got:\n%s", out)
    }
}
