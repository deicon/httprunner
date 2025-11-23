package k6

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	chttp "github.com/deicon/httprunner/src/http"
	"github.com/deicon/httprunner/src/template"
)

// Options controls K6 generation behavior
type Options struct {
	Iterations int
	DelayMS    int
	EnvFile    string
}

// Generate converts parsed .http requests into a K6 JS script.
// It preserves placeholders by converting {{.var}} to ${vars.var} and
// initializes defaults from the provided environment file for discovered keys only.
func Generate(requests []chttp.Request, opts Options) (string, error) {
	if opts.Iterations <= 0 {
		opts.Iterations = 1
	}

	// Discover placeholder keys across all requests
	placeholderKeys := collectPlaceholderKeys(requests)

	// Load defaults from env file (and OS env), filtered to discovered keys only
	defaults := map[string]string{}
	if len(placeholderKeys) > 0 {
		store := template.NewGlobalStore()
		if opts.EnvFile != "" {
			if err := store.LoadEnvFile(opts.EnvFile); err != nil {
				return "", err
			}
		}
		for _, k := range placeholderKeys {
			if v, ok := store.GetAll()[k]; ok {
				defaults[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	var b strings.Builder
	// Header
	b.WriteString("import http from 'k6/http';\n")
	b.WriteString("import { sleep } from 'k6';\n\n")
	b.WriteString(fmt.Sprintf("export const options = {\n  iterations: %d,\n};\n\n", opts.Iterations))

	// Defaults and vars proxy
	b.WriteString("// Defaults derived from environment (.env) for discovered placeholders\n")
	b.WriteString("const defaults = {")
	if len(defaults) > 0 {
		keys := make([]string, 0, len(defaults))
		for k := range defaults {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		first := true
		for _, k := range keys {
			if !first {
				b.WriteString(", ")
			} else {
				first = false
			}
			b.WriteString(jsProp(k))
			b.WriteString(": ")
			b.WriteString(jsStringSingle(defaults[k]))
		}
	}
	b.WriteString("};\n")

	b.WriteString("const vars = new Proxy(Object.assign({}, defaults), {\n")
	b.WriteString("  get: (t, p) => (typeof __ENV !== 'undefined' && __ENV[p] !== undefined ? __ENV[p] : t[p]),\n")
	b.WriteString("  set: (t, p, v) => { t[p] = v; return true; },\n")
	b.WriteString("});\n\n")

	b.WriteString("const client = { global: { set: (k, v) => { vars[k] = v; }, get: (k) => vars[k] } };\n")
	b.WriteString("function safeJson(s) { try { return JSON.parse(s); } catch (_) { return s; } }\n\n")

	// Main function
	b.WriteString("export default function () {\n")
	b.WriteString("  let response;\n")

	delay := opts.DelayMS
	for idx, req := range requests {
		if req.Lifecycle != chttp.LifecycleNone {
			b.WriteString("  // Skipping lifecycle request: ")
			if req.Name != "" {
				b.WriteString(jsStringLiteralComment(req.Name))
			} else {
				b.WriteString(jsStringLiteralComment(req.Verb + " " + req.URL))
			}
			b.WriteString("\n")
			continue
		}

		if strings.TrimSpace(req.PreScript) != "" {
			b.WriteString("  // Pre-script\n")
			b.WriteString(indentJS(req.PreScript, 2))
			b.WriteString("\n")
		}

		hasHTTP := strings.TrimSpace(req.Verb) != "" && strings.TrimSpace(req.URL) != ""
		var respVar string
		if hasHTTP {
			url := toTemplateOrString(req.URL)
			headersObj := buildHeadersObject(req.Headers)

			method := strings.ToUpper(req.Verb)
			switch method {
			case "GET", "DELETE":
				if method == "GET" {
					b.WriteString(fmt.Sprintf("  const httpRes_%d = http.get(%s, { headers: %s });\n", idx, url, headersObj))
				} else {
					b.WriteString(fmt.Sprintf("  const httpRes_%d = http.del(%s, null, { headers: %s });\n", idx, url, headersObj))
				}
			case "POST", "PUT", "PATCH":
				body := toTemplateOrString(req.Body)
				fn := "post"
				if method == "PUT" {
					fn = "put"
				}
				if method == "PATCH" {
					fn = "patch"
				}
				b.WriteString(fmt.Sprintf("  const httpRes_%d = http.%s(%s, %s, { headers: %s });\n", idx, fn, url, body, headersObj))
			default:
				body := toTemplateOrString(req.Body)
				b.WriteString(fmt.Sprintf("  const httpRes_%d = http.request(%s, %s, %s, { headers: %s });\n", idx, jsStringSingle(method), url, body, headersObj))
			}

			b.WriteString(fmt.Sprintf("  const response_%d = { body: safeJson(httpRes_%d.body), status: httpRes_%d.status, headers: httpRes_%d.headers };\n", idx, idx, idx, idx))
			b.WriteString(fmt.Sprintf("  response = response_%d;\n", idx))
			respVar = fmt.Sprintf("response_%d", idx)
		}

		if strings.TrimSpace(req.Script) != "" {
			b.WriteString("  // Post-script\n")
			b.WriteString(indentJS(req.Script, 2))
			b.WriteString("\n")
		}

		if delay > 0 && idx < len(requests)-1 {
			b.WriteString(fmt.Sprintf("  sleep(%g);\n", float64(delay)/1000.0))
		}

		_ = respVar
	}

	b.WriteString("}\n")
	return b.String(), nil
}

var placeholderRe = regexp.MustCompile(`\{\{\s*\.([a-zA-Z0-9_]+)\s*\}\}`)

func collectPlaceholderKeys(requests []chttp.Request) []string {
	keysSet := map[string]struct{}{}
	add := func(s string) {
		for _, m := range placeholderRe.FindAllStringSubmatch(s, -1) {
			if len(m) > 1 {
				keysSet[m[1]] = struct{}{}
			}
		}
	}
	for _, r := range requests {
		add(r.URL)
		for k, v := range r.Headers {
			add(k)
			add(v)
		}
		add(r.Body)
		add(r.PreScript)
		add(r.Script)
	}
	keys := make([]string, 0, len(keysSet))
	for k := range keysSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Replace placeholders with JS template placeholders and choose quoting.
func toTemplateOrString(s string) string {
	if s == "" {
		return jsStringSingle("")
	}
	if placeholderRe.MatchString(s) {
		repl := placeholderRe.ReplaceAllStringFunc(s, func(m string) string {
			sm := placeholderRe.FindStringSubmatch(m)
			if len(sm) > 1 {
				return "${vars." + sm[1] + "}"
			}
			return m
		})
		return jsStringTemplate(repl)
	}
	return jsStringSingle(s)
}

// Build headers object as JS literal
func buildHeadersObject(headers map[string]string) string {
	if len(headers) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := headers[k]
		parts = append(parts, fmt.Sprintf("%s: %s", jsStringSingle(k), toTemplateOrString(v)))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// jsStringSingle encodes as single-quoted string with escapes
func jsStringSingle(s string) string {
	esc := strings.ReplaceAll(s, "\\", "\\\\")
	esc = strings.ReplaceAll(esc, "'", "\\'")
	esc = strings.ReplaceAll(esc, "\n", "\\n")
	esc = strings.ReplaceAll(esc, "\r", "\\r")
	esc = strings.ReplaceAll(esc, "\t", "\\t")
	return "'" + esc + "'"
}

// jsStringTemplate encodes as backtick template, escaping backticks but allowing ${}
func jsStringTemplate(s string) string {
	esc := strings.ReplaceAll(s, "`", "\\`")
	return "`" + esc + "`"
}

// jsProp renders a safe JS identifier or quoted key
func jsProp(k string) string {
	if len(k) > 0 && ((k[0] >= 'a' && k[0] <= 'z') || (k[0] >= 'A' && k[0] <= 'Z') || k[0] == '_') {
		for i := 1; i < len(k); i++ {
			c := k[i]
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
				return jsStringSingle(k)
			}
		}
		return k
	}
	return jsStringSingle(k)
}

// indentJS indents JS code by n spaces at each line
func indentJS(code string, n int) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(code, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "" {
			lines[i] = ""
		} else {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

func jsStringLiteralComment(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
