package template

import (
	"fmt"
	chttp "github.com/deicon/httprunner/http"
	"github.com/deicon/httprunner/metrics"
	"os"
	"path/filepath"
	"testing"
)

func TestNewTemplateEngineWithEnvFile_LoadsEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "API_URL=\"https://example.com\"\nTOKEN=abc123\n# comment\nINVALID_LINE\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	eng, err := NewTemplateEngineWithEnvFile(envPath)
	if err != nil {
		t.Fatalf("NewTemplateEngineWithEnvFile: %v", err)
	}
	if got := eng.GetGlobalStore().Get("API_URL"); got != "https://example.com" {
		t.Fatalf("API_URL not loaded correctly: %v", got)
	}
	if got := eng.GetGlobalStore().Get("TOKEN"); got != "abc123" {
		t.Fatalf("TOKEN not loaded correctly: %v", got)
	}
}

func TestRenderTemplate_Error(t *testing.T) {
	eng := NewTemplateEngine()
	if _, err := eng.RenderTemplate("Hello {{ .MISSING | "); err == nil {
		t.Fatalf("expected template parse error")
	}
}

func TestRequestExecutorErrorPropagation(t *testing.T) {
	eng := NewTemplateEngine()
	eng.RegisterRequestFunction(chttp.Request{Name: "ErrFunc", Verb: "GET", URL: "http://example.com"})
	eng.SetRequestExecutor(func(req chttp.Request) (*Response, error) {
		return nil, fmt.Errorf("boom")
	})

	script := `
        var r = client.errfunc();
        client.global.set("call_success", r.success);
        client.global.set("call_error", r.error);
    `
	if err := eng.ExecuteScript(script, "", 0, 0); err != nil {
		t.Fatalf("ExecuteScript: %v", err)
	}
	if eng.GetGlobalStore().Get("call_success") != false {
		t.Fatalf("expected success=false")
	}
	if eng.GetGlobalStore().Get("call_error") == nil {
		t.Fatalf("expected error message to be set")
	}
}

func TestMetricsAccessInScripts(t *testing.T) {
	eng := NewTemplateEngine()
	mc := metrics.NewMetricsCollector()
	// Record some metrics
	mc.RecordHTTPRequest(10_000_000, 0, 0, 0, 0, 0, 0, false, 100, 200, map[string]string{"url": "/x"})
	mc.RecordCheck(true, nil)
	mc.UpdateVirtualUsers(1, 1)
	eng.SetMetricsCollector(mc)

	script := `
        var m = client.metrics.get("http_req_duration");
        client.global.set("m_name", m.name);
        client.global.set("m_count", m.count);
        var cur = client.metrics.getCurrent("vus");
        client.global.set("vus_current", cur);
        var all = client.metrics.getAll();
        var http_reqs = all["http_reqs"];
        client.global.set("http_reqs_count", http_reqs.count);
    `

	if err := eng.ExecuteScript(script, "", 0, 0); err != nil {
		t.Fatalf("ExecuteScript: %v", err)
	}

	if eng.GetGlobalStore().Get("m_name") != "http_req_duration" {
		t.Fatalf("expected metric name http_req_duration, got %v", eng.GetGlobalStore().Get("m_name"))
	}
	if eng.GetGlobalStore().Get("m_count") == 0 {
		t.Fatalf("expected non-zero count for http_req_duration")
	}
	if eng.GetGlobalStore().Get("vus_current") == nil {
		t.Fatalf("expected current vus metric value to be set")
	}
	if eng.GetGlobalStore().Get("http_reqs_count") == 0 {
		t.Fatalf("expected non-zero http_reqs count")
	}
}
