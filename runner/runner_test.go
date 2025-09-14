package runner

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chttp "github.com/deicon/httprunner/http"
	"github.com/deicon/httprunner/metrics"
	tpl "github.com/deicon/httprunner/template"
)

func scriptOnlyRequest(name string, checks int, fail bool) chttp.Request {
	// Build a script that records a number of checks; last one may fail
	script := ""
	for i := 0; i < checks; i++ {
		if fail && i == checks-1 {
			script += "client.check(\"c\", function(){ return false; }, \"fail\");\n"
		} else {
			script += "client.check(\"c\", function(){ return true; }, \"ok\");\n"
		}
	}
	return chttp.Request{Name: name, Script: script}
}

func TestRunnerRunWithScriptOnlyRequests(t *testing.T) {
	// One VU, two iterations, one script-only normal request per iteration
	reqs := []chttp.Request{
		{Name: "BeforeUser", Lifecycle: chttp.LifecycleBeforeUser, Script: "client.global.set(\"x\", 1)"},
		{Name: "BeforeIteration", Lifecycle: chttp.LifecycleBeforeIteration, Script: "client.global.set(\"y\", 2)"},
		scriptOnlyRequest("DoWork", 1, false),
		{Name: "TeardownIteration", Lifecycle: chttp.LifecycleTeardownIteration, Script: "/*noop*/"},
		{Name: "TeardownUser", Lifecycle: chttp.LifecycleTeardownUser, Script: "/*noop*/"},
	}

	r, err := NewRunner(1, 1, 0, 0, reqs, t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	report, err := r.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.TotalRequests != 1 || report.FailedRequests != 0 {
		t.Fatalf("unexpected totals: %+v", report)
	}
	// Response time distribution should categorize zeros as <100ms
	if report.ResponseTimeDistribution["<100ms"] != 1 {
		t.Fatalf("unexpected distribution: %#v", report.ResponseTimeDistribution)
	}
}

func TestRunnerRunHierarchicalWithConcurrency(t *testing.T) {
	// Two VUs, two iterations, one script-only request per iteration that has one failing check
	reqs := []chttp.Request{scriptOnlyRequest("DoWork", 2, true)}

	r, err := NewRunner(2, 2, 0, 0, reqs, t.TempDir())
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}

	hr, err := r.RunHierarchical()
	if err != nil {
		t.Fatalf("RunHierarchical: %v", err)
	}

	if hr.TotalVirtualUsers != 2 || len(hr.VirtualUserReports) != 2 {
		t.Fatalf("unexpected VU count: %d (%d reports)", hr.TotalVirtualUsers, len(hr.VirtualUserReports))
	}
	// Each VU performs 2 iterations with 1 request each -> 2 requests
	for _, gr := range hr.VirtualUserReports {
		if gr.TotalIterations != 2 || gr.TotalRequests != 2 {
			t.Fatalf("unexpected per-VU totals: %+v", gr)
		}
	}
}

func TestRunnerExecute_TemplatingAndScripts(t *testing.T) {
	// Start a local server to capture headers/body
	var gotAuth, gotTrace, gotBody, gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotTrace = r.Header.Get("X-Trace")
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		gotBody = string(b)
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	te.GetGlobalStore().Set("BASE", ts.URL)
	te.GetGlobalStore().Set("TOKEN", "abc")

	req := chttp.Request{
		Name: "Create",
		Verb: "POST",
		URL:  "{{.BASE}}/api/{{.id}}",
		Headers: map[string]string{
			"Authorization": "Bearer {{.TOKEN}}",
			"X-Trace":       "{{.trace}}",
			"Content-Type":  "application/json",
		},
		Body:      `{"n":"{{.name}}"}`,
		PreScript: `client.global.set("id","v1"); client.global.set("trace","tr-123"); client.global.set("name","bob");`,
		Script:    `client.check("ok", function(){ return response.body.ok === true; }, "not ok");`,
	}

	res := r.execute(req, te, 0, 0)
	if !res.Success || res.StatusCode != http.StatusCreated {
		t.Fatalf("expected success 201, got %+v", res)
	}
	if !strings.HasSuffix(res.URL, "/api/v1") {
		t.Fatalf("templated URL not applied: %s", res.URL)
	}
	if gotAuth != "Bearer abc" || gotTrace != "tr-123" {
		t.Fatalf("templated headers not applied: auth=%q trace=%q", gotAuth, gotTrace)
	}
	if !strings.Contains(gotBody, "bob") {
		t.Fatalf("templated body not applied: %s", gotBody)
	}
	if gotPath != "/api/v1" {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	// Metrics collector should have recorded HTTP timings
	if r.MetricsCollector.GetMetric("http_req_duration").GetCount() == 0 {
		t.Fatalf("http_req_duration metric not recorded")
	}
}

func TestRunnerExecute_FailureAndError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer ts.Close()

	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	te.GetGlobalStore().Set("BASE", ts.URL)

	req := chttp.Request{Name: "X", Verb: "GET", URL: "{{.BASE}}/fail"}
	res := r.execute(req, te, 0, 0)
	if res.Success || res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected failure with 500, got %+v", res)
	}
	if !strings.Contains(res.Error, "HTTP 500") {
		t.Fatalf("expected HTTP 500 error message, got %q", res.Error)
	}
}

func TestRunnerExecuteForFunction_HTTPAndScriptOnly(t *testing.T) {
	// HTTP path
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"k": "v"})
	}))
	defer ts.Close()

	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	te.GetGlobalStore().Set("BASE", ts.URL)

	httpReq := chttp.Request{Name: "HTTP", Verb: "GET", URL: "{{.BASE}}/ok"}
	resp, err := r.executeForFunction(httpReq, te, 1, 2)
	if err != nil || resp.StatusCode == 0 {
		t.Fatalf("executeForFunction http failed: resp=%+v err=%v", resp, err)
	}
	if resp.Body.(map[string]any)["k"].(string) != "v" {
		t.Fatalf("unexpected response body: %+v", resp.Body)
	}

	// Script-only path
	so := chttp.Request{Name: "SO", Script: "/* noop */"}
	resp2, err := r.executeForFunction(so, te, -1, -1)
	if err != nil || resp2.StatusCode != 200 {
		t.Fatalf("script-only executeForFunction failed: %+v err=%v", resp2, err)
	}

	// Give some time for metrics flush to avoid race with rate calculations
	time.Sleep(5 * time.Millisecond)
}

func TestRunnerExecute_PreScriptError(t *testing.T) {
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	// Invalid JS
	req := chttp.Request{Name: "BadPre", Verb: "GET", URL: "http://example.invalid", PreScript: "function(){"}
	res := r.execute(req, te, 0, 0)
	if res.Success || !strings.Contains(res.Error, "error executing pre-request script") {
		t.Fatalf("expected pre-script error, got: %+v", res)
	}
}

func TestRunnerExecute_PostScriptError(t *testing.T) {
	// Server returns valid JSON but script fails
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	te.GetGlobalStore().Set("BASE", ts.URL)
	req := chttp.Request{Name: "BadPost", Verb: "GET", URL: "{{.BASE}}/x", Script: "function(){"}
	res := r.execute(req, te, 0, 0)
	if res.Success || !strings.Contains(res.Error, "error executing script") {
		t.Fatalf("expected post-script error, got: %+v", res)
	}
}

func TestRunnerExecute_HeaderKeyTemplateError(t *testing.T) {
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	req := chttp.Request{Name: "BadHeaderKey", Verb: "GET", URL: "http://example.com", Headers: map[string]string{"{{ .bad |": "x"}}
	res := r.execute(req, te, 0, 0)
	if res.Success || !strings.Contains(res.Error, "error rendering header key template") {
		t.Fatalf("expected header key template error, got: %+v", res)
	}
}

func TestRunnerExecute_HeaderValueTemplateError(t *testing.T) {
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	req := chttp.Request{Name: "BadHeaderVal", Verb: "GET", URL: "http://example.com", Headers: map[string]string{"X": "{{ .bad |"}}
	res := r.execute(req, te, 0, 0)
	if res.Success || !strings.Contains(res.Error, "error rendering header value template") {
		t.Fatalf("expected header value template error, got: %+v", res)
	}
}

func TestRunnerExecute_BodyTemplateError(t *testing.T) {
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	req := chttp.Request{Name: "BadBody", Verb: "POST", URL: "http://example.com", Body: "{{ .nope |"}
	res := r.execute(req, te, 0, 0)
	if res.Success || !strings.Contains(res.Error, "error rendering body template") {
		t.Fatalf("expected body template error, got: %+v", res)
	}
}

func TestRunnerExecute_URLTemplateError(t *testing.T) {
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	req := chttp.Request{Name: "BadURL", Verb: "GET", URL: "{{ .oops |"}
	res := r.execute(req, te, 0, 0)
	if res.Success || !strings.Contains(res.Error, "error rendering URL template") {
		t.Fatalf("expected URL template error, got: %+v", res)
	}
}

func TestRunnerExecute_RequestNameFallback(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	r := &Runner{MetricsCollector: metrics.NewMetricsCollector()}
	te := tpl.NewTemplateEngine()
	te.SetMetricsCollector(r.MetricsCollector)
	te.GetGlobalStore().Set("BASE", ts.URL)
	req := chttp.Request{Name: "", Verb: "GET", URL: "{{.BASE}}/fallback"}
	res := r.execute(req, te, 0, 0)
	if !res.Success {
		t.Fatalf("expected success: %+v", res)
	}
	if !strings.HasPrefix(res.Name, "GET ") || !strings.Contains(res.Name, "/fallback") {
		t.Fatalf("fallback name not applied: %q", res.Name)
	}
}
