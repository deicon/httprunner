package k6

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deicon/httprunner/src/parser"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return p
}

func TestGenerate_K6_SimpleChainAndDelay(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	httpPath := writeTempFile(t, dir, "loadtest.http", `###
GET https://jsonplaceholder.typicode.com/todos/1
Content-Type: application/json

> {%
  client.global.set("userId", response.body.userId);
%}

###
GET https://jsonplaceholder.typicode.com/posts/{{.userId}}
Content-Type: application/json

> {%
    console.log(response.body)
%}
`)

	reqs, err := parser.Parse(httpPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	js, err := Generate(reqs, Options{Iterations: 100, DelayMS: 2000})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Basic assertions
	if !strings.Contains(js, "export const options = {\n  iterations: 100,") {
		t.Fatalf("missing iterations in options, got:\n%s", js)
	}
	if !strings.Contains(js, "sleep(2)") {
		t.Fatalf("missing sleep(2) for 2000ms delay, got:\n%s", js)
	}
	if !strings.Contains(js, "jsonplaceholder.typicode.com/posts/${vars.userId}") {
		t.Fatalf("placeholder not converted to vars proxy, got:\n%s", js)
	}
}

func TestGenerate_K6_EnvDefaultsFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Env file providing default value for TOKEN used in placeholders
	envPath := writeTempFile(t, dir, "local.env", "TOKEN=abc123\n")

	httpPath := writeTempFile(t, dir, "envtest.http", `###
GET https://api.example.com/data/{{.TOKEN}}
Accept: application/json
`)

	reqs, err := parser.Parse(httpPath)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	js, err := Generate(reqs, Options{Iterations: 1, DelayMS: 0, EnvFile: envPath})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Defaults should include TOKEN: 'abc123'
	if !strings.Contains(js, "const defaults = {TOKEN: 'abc123'};") &&
		!strings.Contains(js, "const defaults = { TOKEN: 'abc123' }") &&
		!strings.Contains(js, "const defaults = {TOKEN: 'abc123'}") {
		t.Fatalf("missing defaults token value, got:\n%s", js)
	}

	// URL should use vars proxy placeholder
	if !strings.Contains(js, "api.example.com/data/${vars.TOKEN}") {
		t.Fatalf("missing runtime placeholder in URL, got:\n%s", js)
	}
}
