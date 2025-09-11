package template

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	chttp "github.com/deicon/httprunner/http"
	"github.com/deicon/httprunner/metrics"
	"github.com/deicon/httprunner/reporting/types"
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

// Response represents a simplified HTTP response for JavaScript access
type Response struct {
	StatusCode int               `json:"status_code"`
	Body       interface{}       `json:"body"`
	Headers    map[string]string `json:"headers"`
}

// RequestExecutor is a function type for executing HTTP requests
type RequestExecutor func(request chttp.Request) (*Response, error)

// TemplateEngine handles template rendering and JavaScript execution
type Engine struct {
	globalStore      *GlobalStore
	checks           []types.CheckResult
	checksMu         sync.Mutex
	metricsCollector *metrics.MetricsCollector
	requestFunctions map[string]chttp.Request
	requestExecutor  RequestExecutor
	// Persistent VM for maintaining JavaScript state between executions
	vm   *goja.Runtime
	vmMu sync.Mutex
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() *Engine {
	return &Engine{
		globalStore:      NewGlobalStore(),
		checks:           make([]types.CheckResult, 0),
		requestFunctions: make(map[string]chttp.Request),
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
		globalStore:      store,
		checks:           make([]types.CheckResult, 0),
		requestFunctions: make(map[string]chttp.Request),
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

// initializeVM initializes the JavaScript VM with client objects
func (te *Engine) initializeVM(virtualUserID, iterationID int) *goja.Runtime {
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
		te.checks = append(te.checks, types.CheckResult{
			Name:           name,
			Success:        success,
			FailureMessage: failureMessage,
			Timestamp:      time.Now(),
		})
	})

	// Add metrics access if available
	if te.metricsCollector != nil {
		metricsObj := vm.NewObject()

		// Add method to get metric summary
		_ = metricsObj.Set("get", func(metricName string) interface{} {
			metric := te.metricsCollector.GetMetric(metricName)
			if metric == nil {
				return nil
			}
			summary := metric.GetSummary()

			// Convert to JavaScript-friendly object
			jsObj := vm.NewObject()
			_ = jsObj.Set("name", summary.Name)
			_ = jsObj.Set("type", string(summary.Type))
			_ = jsObj.Set("count", summary.Count)
			_ = jsObj.Set("sum", summary.Sum)
			_ = jsObj.Set("average", summary.Average)
			_ = jsObj.Set("min", summary.Min)
			_ = jsObj.Set("max", summary.Max)
			_ = jsObj.Set("p50", summary.P50)
			_ = jsObj.Set("p90", summary.P90)
			_ = jsObj.Set("p95", summary.P95)
			_ = jsObj.Set("p99", summary.P99)
			_ = jsObj.Set("latest_value", summary.LatestValue)

			return jsObj
		})

		// Add method to get all metric summaries
		_ = metricsObj.Set("getAll", func() interface{} {
			summaries := te.metricsCollector.GetSummaries()

			// Convert to JavaScript-friendly object
			jsObj := vm.NewObject()
			for name, summary := range summaries {
				metricObj := vm.NewObject()
				_ = metricObj.Set("name", summary.Name)
				_ = metricObj.Set("type", string(summary.Type))
				_ = metricObj.Set("count", summary.Count)
				_ = metricObj.Set("sum", summary.Sum)
				_ = metricObj.Set("average", summary.Average)
				_ = metricObj.Set("min", summary.Min)
				_ = metricObj.Set("max", summary.Max)
				_ = metricObj.Set("p50", summary.P50)
				_ = metricObj.Set("p90", summary.P90)
				_ = metricObj.Set("p95", summary.P95)
				_ = metricObj.Set("p99", summary.P99)
				_ = metricObj.Set("latest_value", summary.LatestValue)

				_ = jsObj.Set(name, metricObj)
			}

			return jsObj
		})

		// Add method to get current metric value
		_ = metricsObj.Set("getCurrent", func(metricName string) interface{} {
			metric := te.metricsCollector.GetMetric(metricName)
			if metric == nil {
				return nil
			}
			latest := metric.GetLatest()
			if latest == nil {
				return nil
			}
			return latest.Value
		})

		_ = clientObj.Set("metrics", metricsObj)
	}

	_ = clientObj.Set("global", globalObj)

	// Create context object with user and iteration IDs
	contextObj := vm.NewObject()
	_ = contextObj.Set("userId", virtualUserID)
	_ = contextObj.Set("iterationId", iterationID)
	_ = vm.Set("context", contextObj)

	// Add named request functions to client object
	for functionName, request := range te.requestFunctions {
		// Capture the request in closure to avoid variable capture issues
		requestCopy := request
		_ = clientObj.Set(functionName, func() interface{} {
			if te.requestExecutor != nil {
				response, err := te.requestExecutor(requestCopy)
				if err != nil {
					// Return error information
					errorObj := vm.NewObject()
					_ = errorObj.Set("error", err.Error())
					_ = errorObj.Set("success", false)
					return errorObj
				}
				// Convert response to JavaScript object
				responseObj := vm.NewObject()
				_ = responseObj.Set("body", response.Body)
				_ = responseObj.Set("status_code", response.StatusCode)
				_ = responseObj.Set("headers", response.Headers)
				_ = responseObj.Set("success", true)
				return responseObj
			}
			// If no executor is set, return empty response
			emptyObj := vm.NewObject()
			_ = emptyObj.Set("error", "Request executor not available")
			_ = emptyObj.Set("success", false)
			return emptyObj
		})
	}

	_ = vm.Set("client", clientObj)

	// Add console support for logging
	consoleObj := vm.NewObject()
	_ = consoleObj.Set("log", func(messages ...interface{}) {
		for i, msg := range messages {
			if i > 0 {
				fmt.Print(" ")
			}
			fmt.Print(msg)
		}
		fmt.Println()
	})
	_ = vm.Set("console", consoleObj)

	return vm
}

// ExecuteScript executes JavaScript code with access to global store and response
func (te *Engine) ExecuteScript(script string, responseBody string, virtualUserID, iterationID int) error {
	te.vmMu.Lock()
	defer te.vmMu.Unlock()

	// Initialize VM if not already done
	if te.vm == nil {
		te.vm = te.initializeVM(virtualUserID, iterationID)
	}

	vm := te.vm

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

// SetMetricsCollector sets the metrics collector for accessing metrics in scripts
func (te *Engine) SetMetricsCollector(collector *metrics.MetricsCollector) {
	te.metricsCollector = collector
}

// GetChecks returns the current checks and clears the internal list
func (te *Engine) GetChecks() []types.CheckResult {
	te.checksMu.Lock()
	defer te.checksMu.Unlock()

	checks := make([]types.CheckResult, len(te.checks))
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

// RegisterRequestFunction registers a named request as a callable function
func (te *Engine) RegisterRequestFunction(request chttp.Request) {
	if request.Name != "" {
		functionName := convertNameToFunctionName(request.Name)
		te.requestFunctions[functionName] = request
	}
}

// SetRequestExecutor sets the function used to execute HTTP requests
func (te *Engine) SetRequestExecutor(executor RequestExecutor) {
	te.requestExecutor = executor
}

// convertNameToFunctionName converts a request name to a JavaScript function name
// Example: "Create User" -> "create_user"
func convertNameToFunctionName(name string) string {
	// Convert to lowercase and replace spaces with underscores
	functionName := strings.ToLower(name)
	// Replace any non-alphanumeric characters with underscores
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	functionName = reg.ReplaceAllString(functionName, "_")
	// Remove consecutive underscores
	reg = regexp.MustCompile(`_+`)
	functionName = reg.ReplaceAllString(functionName, "_")
	// Remove leading/trailing underscores
	functionName = strings.Trim(functionName, "_")
	return functionName
}
