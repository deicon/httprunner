package template

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	chttp "github.com/deicon/httprunner/src/http"
	"github.com/deicon/httprunner/src/metrics"
	"github.com/deicon/httprunner/src/reporting/types"
	"github.com/dop251/goja"
)

// AssertionError represents an assertion failure with custom HTTP status code
type AssertionError struct {
	Message    string
	StatusCode int
}

func (e *AssertionError) Error() string {
	return e.Message
}

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

// ReplaceAll replaces the contents of the store with the provided values
func (gs *GlobalStore) ReplaceAll(values map[string]interface{}) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if values == nil {
		gs.data = make(map[string]interface{})
		return
	}

	gs.data = make(map[string]interface{}, len(values))
	for k, v := range values {
		gs.data[k] = v
	}
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
	// Runtime mode selection and optional Node process
	runtimeMode RuntimeMode
	nodeRuntime *nodeRuntime
	// Additional search paths for resolving Node.js modules
	nodeRequirePaths []string
}

// RuntimeMode represents the active JavaScript runtime implementation
type RuntimeMode string

const (
	// RuntimeModeGoja executes JavaScript using the embedded Goja engine
	RuntimeModeGoja RuntimeMode = "goja"
	// RuntimeModeNode executes JavaScript via an external Node.js worker
	RuntimeModeNode RuntimeMode = "node"
)

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() *Engine {
	return &Engine{
		globalStore:      NewGlobalStore(),
		checks:           make([]types.CheckResult, 0),
		requestFunctions: make(map[string]chttp.Request),
		runtimeMode:      RuntimeModeGoja,
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
		runtimeMode:      RuntimeModeGoja,
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

	// Add assert method
	_ = clientObj.Set("assert", func(conditionHandler func() bool, failureMessage string, statusCode int) {
		success := conditionHandler()
		if !success {
			// Create and throw a JavaScript exception directly
			exc := vm.NewTypeError(fmt.Sprintf("ASSERTION_ERROR:%d:%s", statusCode, failureMessage))
			panic(exc)
		}
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

	// Add simple sleep function
	_ = vm.Set("sleep", func(millis float64) {
		time.Sleep(time.Duration(millis * float64(time.Millisecond)))
	})

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
	if te.runtimeMode == RuntimeModeNode {
		return te.executeScriptNode(script, responseBody, virtualUserID, iterationID)
	}
	return te.executeScriptGoja(script, responseBody, virtualUserID, iterationID)
}

func (te *Engine) executeScriptGoja(script string, responseBody string, virtualUserID, iterationID int) error {
	te.vmMu.Lock()
	defer te.vmMu.Unlock()

	// Handle assertion panics and convert them to errors
	var execError error
	defer func() {
		if r := recover(); r != nil {
			if assertErr, ok := r.(*AssertionError); ok {
				// Convert assertion error to regular error
				execError = assertErr
				return
			}
			// Re-panic if it's not our assertion error
			panic(r)
		}
	}()

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
		// Check if the error is our custom assertion error by parsing the message
		errMsg := err.Error()
		if strings.Contains(errMsg, "ASSERTION_ERROR:") {
			// Extract status code and message from the error
			if parts := strings.SplitN(errMsg, "ASSERTION_ERROR:", 2); len(parts) == 2 {
				if subParts := strings.SplitN(parts[1], ":", 2); len(subParts) == 2 {
					statusCode := 500 // default
					if code, convErr := strconv.Atoi(strings.TrimSpace(subParts[0])); convErr == nil {
						statusCode = code
					}
					message := strings.TrimSpace(subParts[1])
					return &AssertionError{
						Message:    message,
						StatusCode: statusCode,
					}
				}
			}
		}
		return fmt.Errorf("script execution error: %v", err)
	}

	// Return assertion error if one occurred during execution
	if execError != nil {
		return execError
	}

	return nil
}

func (te *Engine) executeScriptNode(script string, responseBody string, virtualUserID, iterationID int) error {
	te.vmMu.Lock()
	runtime := te.nodeRuntime
	if runtime == nil {
		var err error
		runtime, err = newNodeRuntime()
		if err != nil {
			te.vmMu.Unlock()
			return fmt.Errorf("failed to start Node.js runtime: %w", err)
		}
		te.nodeRuntime = runtime
	}

	reqFuncs := make(map[string]chttp.Request, len(te.requestFunctions))
	for name, req := range te.requestFunctions {
		reqFuncs[name] = req
	}
	requestExecutor := te.requestExecutor
	requirePaths := append([]string(nil), te.nodeRequirePaths...)
	te.vmMu.Unlock()

	// Prepare response data similar to Goja runtime
	var responseData interface{}
	if responseBody != "" {
		if err := json.Unmarshal([]byte(responseBody), &responseData); err != nil {
			responseData = map[string]interface{}{
				"body": responseBody,
			}
		}
	} else {
		responseData = map[string]interface{}{}
	}

	reqPayload := nodeExecuteRequest{
		Type:         "execute",
		Script:       script,
		ResponseBody: responseData,
		Context: map[string]interface{}{
			"userId":      virtualUserID,
			"iterationId": iterationID,
		},
		Globals: te.globalStore.GetAll(),
	}

	if len(reqFuncs) > 0 {
		reqPayload.RequestFunctions = make([]string, 0, len(reqFuncs))
		for name := range reqFuncs {
			reqPayload.RequestFunctions = append(reqPayload.RequestFunctions, name)
		}
	}

	if len(requirePaths) > 0 {
		reqPayload.RequirePaths = append(reqPayload.RequirePaths, requirePaths...)
	}

	handler := func(name string, args []interface{}) (*nodeRequestResponse, error) {
		req, ok := reqFuncs[name]
		if !ok {
			return &nodeRequestResponse{
				Success: false,
				Error:   fmt.Sprintf("unknown request function: %s", name),
			}, nil
		}

		if requestExecutor == nil {
			return &nodeRequestResponse{
				Success: false,
				Error:   "request executor not available",
			}, nil
		}

		resp, execErr := requestExecutor(req)
		result := &nodeRequestResponse{
			Success: execErr == nil,
		}

		if resp != nil {
			result.StatusCode = resp.StatusCode
			result.Headers = resp.Headers
			result.Body = resp.Body
		}

		if execErr != nil {
			result.Error = execErr.Error()
		}

		if resp == nil && execErr == nil {
			result.Success = false
			result.Error = "request executor returned no response"
		}

		return result, nil
	}

	response, err := runtime.Execute(reqPayload, handler)
	te.vmMu.Lock()
	defer te.vmMu.Unlock()

	if err != nil {
		if te.nodeRuntime != nil {
			_ = te.nodeRuntime.Close()
			te.nodeRuntime = nil
		}
		return fmt.Errorf("node runtime execution failed: %w", err)
	}

	if response.Globals != nil {
		te.globalStore.ReplaceAll(response.Globals)
	}

	if len(response.Checks) > 0 {
		now := time.Now()
		te.checksMu.Lock()
		for _, check := range response.Checks {
			te.checks = append(te.checks, types.CheckResult{
				Name:           check.Name,
				Success:        check.Success,
				FailureMessage: check.FailureMessage,
				Timestamp:      now,
			})
		}
		te.checksMu.Unlock()
	}

	for _, entry := range response.Logs {
		if entry.Message != "" {
			fmt.Println(entry.Message)
		}
	}

	switch response.Type {
	case nodeResponseTypeResult:
		return nil
	case nodeResponseTypeAssertion:
		if response.Assertion == nil {
			return fmt.Errorf("node runtime assertion missing details")
		}
		statusCode := response.Assertion.StatusCode
		if statusCode == 0 {
			statusCode = 500
		}
		return &AssertionError{
			Message:    response.Assertion.Message,
			StatusCode: statusCode,
		}
	case nodeResponseTypeError:
		if response.Error != nil {
			if response.Error.Stack != "" {
				return fmt.Errorf("node runtime error: %s\n%s", response.Error.Message, response.Error.Stack)
			}
			return fmt.Errorf("node runtime error: %s", response.Error.Message)
		}
		return fmt.Errorf("node runtime reported an error without details")
	default:
		return fmt.Errorf("unexpected node runtime response type: %s", response.Type)
	}
}

// GetGlobalStore returns the global store for external access
func (te *Engine) GetGlobalStore() *GlobalStore {
	return te.globalStore
}

// SetMetricsCollector sets the metrics collector for accessing metrics in scripts
func (te *Engine) SetMetricsCollector(collector *metrics.MetricsCollector) {
	te.metricsCollector = collector
}

// SetRuntimeMode switches the engine to the provided JavaScript runtime
func (te *Engine) SetRuntimeMode(mode RuntimeMode) {
	te.runtimeMode = mode
}

// SetNodeRequirePaths configures additional directories for resolving Node.js modules
func (te *Engine) SetNodeRequirePaths(paths []string) {
	te.nodeRequirePaths = paths
}

// Close releases resources associated with the engine
func (te *Engine) Close() error {
	te.vmMu.Lock()
	defer te.vmMu.Unlock()

	if te.nodeRuntime != nil {
		err := te.nodeRuntime.Close()
		te.nodeRuntime = nil
		return err
	}
	return nil
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
