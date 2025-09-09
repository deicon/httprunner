package template

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/deicon/httprunner/reporting"
	"github.com/dop251/goja"
)

// GlobalStore manages global variables shared across requests
type GlobalStore struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewGlobalStore creates a new global store with environment variables
func NewGlobalStore() *GlobalStore {
	store := &GlobalStore{
		data: make(map[string]interface{}),
	}

	// Load environment variables
	for _, env := range os.Environ() {
		if len(env) > 0 {
			for i, c := range env {
				if c == '=' {
					key := env[:i]
					value := env[i+1:]
					store.data[key] = value
					break
				}
			}
		}
	}

	return store
}

// LoadEnvFile loads environment variables from a .env file
func (gs *GlobalStore) LoadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("error opening .env file: %v", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first '=' to split key and value
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}

		gs.Set(key, value)
	}

	return scanner.Err()
}

// Set stores a value in the global store
func (gs *GlobalStore) Set(key string, value interface{}) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	gs.data[key] = value
}

// Get retrieves a value from the global store
func (gs *GlobalStore) Get(key string) interface{} {
	gs.mu.RLock()
	defer gs.mu.RUnlock()
	return gs.data[key]
}

// GetAll returns a copy of all data for template rendering
func (gs *GlobalStore) GetAll() map[string]interface{} {
	gs.mu.RLock()
	defer gs.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range gs.data {
		result[k] = v
	}
	return result
}

// TemplateEngine handles template rendering and JavaScript execution
type Engine struct {
	globalStore *GlobalStore
	checks      []reporting.CheckResult
	checksMu    sync.Mutex
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() *Engine {
	return &Engine{
		globalStore: NewGlobalStore(),
		checks:      make([]reporting.CheckResult, 0),
	}
}

// NewTemplateEngineWithEnvFile creates a new template engine with env file loaded
func NewTemplateEngineWithEnvFile(envFile string) (*Engine, error) {
	store := NewGlobalStore()
	if envFile != "" {
		if err := store.LoadEnvFile(envFile); err != nil {
			return nil, err
		}
	}
	return &Engine{
		globalStore: store,
		checks:      make([]reporting.CheckResult, 0),
	}, nil
}

// RenderTemplate renders a template string with global variables
func (te *Engine) RenderTemplate(templateStr string) (string, error) {
	tmpl, err := template.New("request").Funcs(sprig.GenericFuncMap()).Parse(templateStr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, te.globalStore.GetAll()); err != nil {
		return "", fmt.Errorf("template execution error: %v", err)
	}

	return buf.String(), nil
}

// ExecuteScript executes JavaScript code with access to global store and response
func (te *Engine) ExecuteScript(script string, responseBody string) error {
	vm := goja.New()

	// Create client.global object
	clientObj := vm.NewObject()
	globalObj := vm.NewObject()

	// Add set method
	_ = globalObj.Set("set", func(key string, value interface{}) {
		te.globalStore.Set(key, value)
	})

	// Add get method
	_ = globalObj.Set("get", func(key string) interface{} {
		return te.globalStore.Get(key)
	})

	// Add check method
	_ = clientObj.Set("check", func(name string, checkHandler func() bool, failureMessage string) {
		success := checkHandler()
		te.checksMu.Lock()
		defer te.checksMu.Unlock()
		te.checks = append(te.checks, reporting.CheckResult{
			Name:           name,
			Success:        success,
			FailureMessage: failureMessage,
			Timestamp:      time.Now(),
		})
	})

	_ = clientObj.Set("global", globalObj)
	_ = vm.Set("client", clientObj)

	// Parse response body as JSON if possible
	var responseData interface{}
	if responseBody != "" {
		if err := json.Unmarshal([]byte(responseBody), &responseData); err != nil {
			// If JSON parsing fails, use raw string
			responseData = map[string]interface{}{
				"body": responseBody,
			}
		}
	} else {
		responseData = map[string]interface{}{}
	}

	// Create response object
	responseObj := vm.NewObject()
	_ = responseObj.Set("body", responseData)
	_ = vm.Set("response", responseObj)

	// Execute the script
	_, err := vm.RunString(script)
	if err != nil {
		return fmt.Errorf("script execution error: %v", err)
	}

	return nil
}

// GetGlobalStore returns the global store for external access
func (te *Engine) GetGlobalStore() *GlobalStore {
	return te.globalStore
}

// GetChecks returns the current checks and clears the internal list
func (te *Engine) GetChecks() []reporting.CheckResult {
	te.checksMu.Lock()
	defer te.checksMu.Unlock()

	checks := make([]reporting.CheckResult, len(te.checks))
	copy(checks, te.checks)
	te.checks = te.checks[:0] // Clear the slice

	return checks
}

// ClearChecks clears the internal check results
func (te *Engine) ClearChecks() {
	te.checksMu.Lock()
	defer te.checksMu.Unlock()
	te.checks = te.checks[:0]
}
