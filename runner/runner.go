package runner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"net/http/httptrace"
	"sync"
	"time"

	chttp "github.com/deicon/httprunner/http"
	"github.com/deicon/httprunner/metrics"
	"github.com/deicon/httprunner/reporting"
	"github.com/deicon/httprunner/reporting/streaming"
	"github.com/deicon/httprunner/reporting/types"
	"github.com/deicon/httprunner/template"
)

// Runner executes HTTP requests
type Runner struct {
	Concurrency        int
	Iterations         int
	Runtime            int // Runtime in seconds (0 means use iterations)
	Delay              int
	Requests           []chttp.Request
	envFile            string
	StreamingCollector *streaming.StreamingCollector
	OutputDir          string
	MetricsCollector   *metrics.MetricsCollector
	Verbose            bool
	// Categorized requests by lifecycle
	beforeUserRequests        []chttp.Request
	beforeIterationRequests   []chttp.Request
	teardownUserRequests      []chttp.Request
	teardownIterationRequests []chttp.Request
	normalRequests            []chttp.Request
}

// NewRunner creates a new Runner with file streaming for memory efficiency
func NewRunner(concurrency, iterations, runtime, delay int, requests []chttp.Request, outputDir string) (*Runner, error) {
	streamingCollector, err := streaming.NewStreamingCollector(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming collector: %v", err)
	}

	metricsCollector := metrics.NewMetricsCollector()

	runner := &Runner{
		Concurrency:        concurrency,
		Iterations:         iterations,
		Runtime:            runtime,
		Delay:              delay,
		Requests:           requests,
		StreamingCollector: streamingCollector,
		OutputDir:          outputDir,
		MetricsCollector:   metricsCollector,
	}

	runner.categorizeRequests()
	return runner, nil
}

// NewRunnerWithEnvFile creates a new Runner with streaming and env file support
func NewRunnerWithEnvFile(concurrency, iterations, runtime, delay int, requests []chttp.Request, envFile, outputDir string) (*Runner, error) {
	streamingCollector, err := streaming.NewStreamingCollector(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming collector: %v", err)
	}

	metricsCollector := metrics.NewMetricsCollector()

	runner := &Runner{
		Concurrency:        concurrency,
		Iterations:         iterations,
		Runtime:            runtime,
		Delay:              delay,
		Requests:           requests,
		envFile:            envFile,
		StreamingCollector: streamingCollector,
		OutputDir:          outputDir,
		MetricsCollector:   metricsCollector,
	}

	runner.categorizeRequests()
	return runner, nil
}

// SetVerbose enables or disables verbose mode
func (r *Runner) SetVerbose(verbose bool) {
	r.Verbose = verbose
}

// Run executes requests with file streaming to reduce memory usage
func (r *Runner) Run() (*types.Report, error) {
	if r.StreamingCollector == nil {
		return nil, fmt.Errorf("streaming collector not initialized, use NewStreamingRunner")
	}

	startTime := r.StreamingCollector.GetStartTime()

	if err := r.executeWithStreaming(); err != nil {
		return nil, err
	}

	// Generate report from file
	fileReporter := reporting.NewFileReporter(r.StreamingCollector.GetResultsFilePath())
	report, err := fileReporter.GenerateReport(startTime)
	if err != nil {
		return nil, err
	}

	// Add metrics summaries to the report
	report.MetricsSummaries = r.MetricsCollector.GetSummaries()
	report.TotalVirtualUsers = r.Concurrency
	report.RuntimeSeconds = r.MetricsCollector.GetRuntimeSeconds()
	report.PerVUMetrics = r.MetricsCollector.GetPerVUMetrics(r.Concurrency)
	report.PerIterationMetrics = r.MetricsCollector.GetPerIterationMetrics()

	return report, nil
}

// RunHierarchical executes requests with file streaming and returns hierarchical report
func (r *Runner) RunHierarchical() (*types.HierarchicalReport, error) {
	if r.StreamingCollector == nil {
		return nil, fmt.Errorf("streaming collector not initialized, use NewStreamingRunner")
	}

	startTime := r.StreamingCollector.GetStartTime()

	if err := r.executeWithStreaming(); err != nil {
		return nil, err
	}

	// Generate hierarchical report from file
	fileReporter := reporting.NewFileReporter(r.StreamingCollector.GetResultsFilePath())
	hierarchicalReport, err := fileReporter.GenerateHierarchicalReport(startTime)
	if err != nil {
		return nil, err
	}

	// Add metrics summaries to the summary report
	hierarchicalReport.Summary.MetricsSummaries = r.MetricsCollector.GetSummaries()

	return hierarchicalReport, nil
}

// executeWithStreaming contains the common streaming execution pattern
func (r *Runner) executeWithStreaming() error {
	var wg sync.WaitGroup
	resultChan := make(chan types.RequestResult, 1000)

	// Create a shared template engine for @BeforeUser scripts
	globalTemplateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
	globalTemplateEngine.SetMetricsCollector(r.MetricsCollector)

	// Register all named requests as callable functions
	for _, req := range r.Requests {
		globalTemplateEngine.RegisterRequestFunction(req)
	}

	// Set up request executor for function calls
	globalTemplateEngine.SetRequestExecutor(func(request chttp.Request) (*template.Response, error) {
		// Execute the request and return a simplified response
		responseData, err := r.executeForFunction(request, globalTemplateEngine, -1, -1) // Use -1 to indicate system execution
		return responseData, err
	})

	// Execute @BeforeUser scripts once before starting workers
	for _, req := range r.beforeUserRequests {
		if req.PreScript != "" || req.Script != "" {
			script := req.PreScript + "\n" + req.Script
			if err := globalTemplateEngine.ExecuteScript(script, "", -1, -1); err != nil {
				return fmt.Errorf("error executing @BeforeUser script for %s: %v", req.Name, err)
			}
		}
	}

	wg.Add(r.Concurrency)

	// Update VU metrics
	r.MetricsCollector.UpdateVirtualUsers(r.Concurrency, r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()

			// Create template engine for this worker, inheriting global state
			templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
			templateEngine.SetMetricsCollector(r.MetricsCollector)

			// Copy global state to worker template ein inngine
			globalStore := globalTemplateEngine.GetGlobalStore()
			for k, v := range globalStore.GetAll() {
				templateEngine.GetGlobalStore().Set(k, v)
			}

			// Register named request functions for this worker too
			for _, req := range r.Requests {
				templateEngine.RegisterRequestFunction(req)
			}

			// Set up request executor for this worker
			templateEngine.SetRequestExecutor(func(request chttp.Request) (*template.Response, error) {
				responseData, err := r.executeForFunction(request, templateEngine, workerID, -1)
				return responseData, err
			})

			// Determine execution mode: time-based or iteration-based
			var endTime time.Time
			var useTimeBasedExecution bool
			if r.Runtime > 0 {
				useTimeBasedExecution = true
				endTime = time.Now().Add(time.Duration(r.Runtime) * time.Second)
			}

			j := 0 // iteration counter
			for {
				// Check if we should stop based on execution mode
				if useTimeBasedExecution {
					if time.Now().After(endTime) {
						break
					}
				} else {
					if j >= r.Iterations {
						break
					}
				}

				iterationStart := time.Now()

				// Execute @BeforeIteration scripts before each iteration
				for _, req := range r.beforeIterationRequests {
					if req.PreScript != "" || req.Script != "" {
						script := req.PreScript + "\n" + req.Script
						if err := templateEngine.ExecuteScript(script, "", workerID, j); err != nil {
							fmt.Printf("[Worker %d] Error executing @BeforeIteration script for %s: %v\n", workerID, req.Name, err)
						}
					}
				}

				// Execute normal requests
				shouldBreak := false
				for _, req := range r.normalRequests {
					// Check time limit before each request in time-based mode
					if useTimeBasedExecution && time.Now().After(endTime) {
						shouldBreak = true
						break
					}

					result := r.execute(req, templateEngine, workerID, j)
					resultChan <- result
					if !result.Success {
						fmt.Printf("[Worker %d] Error: %v - Stopping iteration %d\n", workerID, result.Error, j+1)
						shouldBreak = true
						break
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
				}

				if shouldBreak {
					break
				}

				// Execute @TeardownIteration scripts after each iteration
				for _, req := range r.teardownIterationRequests {
					if req.PreScript != "" || req.Script != "" {
						script := req.PreScript + "\n" + req.Script
						if err := templateEngine.ExecuteScript(script, "", workerID, j); err != nil {
							fmt.Printf("[Worker %d] Error executing @TeardownIteration script for %s: %v\n", workerID, req.Name, err)
						}
					}
				}

				// Record iteration metrics
				iterationDuration := time.Since(iterationStart)
				tags := map[string]string{
					"vu":        fmt.Sprintf("%d", workerID),
					"iteration": fmt.Sprintf("%d", j),
				}
				r.MetricsCollector.RecordIteration(iterationDuration, tags)

				j++ // increment iteration counter
			}

			// Execute @TeardownUser scripts once after all iterations for this worker
			for _, req := range r.teardownUserRequests {
				if req.PreScript != "" || req.Script != "" {
					script := req.PreScript + "\n" + req.Script
					if err := templateEngine.ExecuteScript(script, "", workerID, -1); err != nil {
						fmt.Printf("[Worker %d] Error executing @TeardownUser script for %s: %v\n", workerID, req.Name, err)
					}
				}
			}
		}(i)
	}

	// Stream results to file in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Stream all results to file
	for result := range resultChan {
		if err := r.StreamingCollector.AddResult(result); err != nil {
			return fmt.Errorf("failed to stream result: %v", err)
		}
	}

	// Mark end time for rate calculations
	r.MetricsCollector.SetEndTime()

	// Close streaming collector
	if err := r.StreamingCollector.Close(); err != nil {
		return fmt.Errorf("failed to close streaming collector: %v", err)
	}

	return nil
}

// categorizeRequests separates requests by their lifecycle annotations
func (r *Runner) categorizeRequests() {
	r.beforeUserRequests = make([]chttp.Request, 0)
	r.beforeIterationRequests = make([]chttp.Request, 0)
	r.teardownUserRequests = make([]chttp.Request, 0)
	r.teardownIterationRequests = make([]chttp.Request, 0)
	r.normalRequests = make([]chttp.Request, 0)

	for _, req := range r.Requests {
		switch req.Lifecycle {
		case chttp.LifecycleBeforeUser:
			r.beforeUserRequests = append(r.beforeUserRequests, req)
		case chttp.LifecycleBeforeIteration:
			r.beforeIterationRequests = append(r.beforeIterationRequests, req)
		case chttp.LifecycleTeardownUser:
			r.teardownUserRequests = append(r.teardownUserRequests, req)
		case chttp.LifecycleTeardownIteration:
			r.teardownIterationRequests = append(r.teardownIterationRequests, req)
		default:
			// Normal requests (no lifecycle annotation)
			r.normalRequests = append(r.normalRequests, req)
		}
	}
}

func (r *Runner) execute(req chttp.Request, te *template.Engine, virtualUserId, iterationID int) types.RequestResult {
	result := types.RequestResult{
		Name:          req.Name,
		Verb:          req.Verb,
		URL:           req.URL,
		Timestamp:     time.Now(),
		VirtualUserID: virtualUserId,
		IterationID:   iterationID,
	}

	// Execute pre-request script if present
	if req.PreScript != "" {
		if err := te.ExecuteScript(req.PreScript, "", virtualUserId, iterationID); err != nil {
			result.Success = false
			if assertErr, ok := err.(*template.AssertionError); ok {
				result.StatusCode = assertErr.StatusCode
				result.Error = assertErr.Message
			} else {
				result.Error = fmt.Sprintf("error executing pre-request script: %v", err)
			}
			return result
		}
	}

	// Handle script-only requests (no HTTP verb/URL)
	if req.Verb == "" && req.URL == "" {
		// For script-only requests, just execute the post-request script and return
		if req.Script != "" {
			if err := te.ExecuteScript(req.Script, "", virtualUserId, iterationID); err != nil {
				result.Success = false
				if assertErr, ok := err.(*template.AssertionError); ok {
					result.StatusCode = assertErr.StatusCode
					result.Error = assertErr.Message
				} else {
					result.Error = fmt.Sprintf("error executing script: %v", err)
				}
				return result
			}
		}
		result.Success = true
		result.StatusCode = 200 // Virtual success status for script-only requests
		return result
	}

	// Render templates in URL
	renderedURL, err := te.RenderTemplate(req.URL)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("error rendering URL template: %v", err)
		return result
	}
	result.URL = renderedURL

	// Render templates in headers
	renderedHeaders := make(map[string]string)
	for key, value := range req.Headers {
		renderedKey, err := te.RenderTemplate(key)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("error rendering header key template: %v", err)
			return result
		}
		renderedValue, err := te.RenderTemplate(value)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("error rendering header value template: %v", err)
			return result
		}
		renderedHeaders[renderedKey] = renderedValue
	}

	// Render templates in body
	renderedBody, err := te.RenderTemplate(req.Body)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("error rendering body template: %v", err)
		return result
	}

	// Create and execute HTTP request
	client := &nethttp.Client{}
	request, err := nethttp.NewRequest(req.Verb, renderedURL, bytes.NewBufferString(renderedBody))
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	for key, value := range renderedHeaders {
		request.Header.Set(key, value)
	}

	// Initialize trace metrics
	var (
		startTime    = time.Now()
		dnsStart     time.Time
		dnsEnd       time.Time
		connectStart time.Time
		connectEnd   time.Time
		tlsStart     time.Time
		tlsEnd       time.Time
		gotFirstByte time.Time
		wroteRequest time.Time
		gotResponse  time.Time
	)

	// Create HTTP trace to capture real timing metrics
	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			dnsEnd = time.Now()
		},
		ConnectStart: func(_, _ string) {
			connectStart = time.Now()
		},
		ConnectDone: func(_, _ string, _ error) {
			connectEnd = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			tlsEnd = time.Now()
		},
		WroteRequest: func(_ httptrace.WroteRequestInfo) {
			wroteRequest = time.Now()
		},
		GotFirstResponseByte: func() {
			gotFirstByte = time.Now()
		},
	}

	// Create context with trace
	ctx := httptrace.WithClientTrace(context.Background(), trace)
	request = request.WithContext(ctx)

	// Execute HTTP request
	resp, err := client.Do(request)
	gotResponse = time.Now()

	// Calculate timing metrics
	duration := gotResponse.Sub(startTime)
	// Ensure the measured duration is recorded on the result for reporting
	result.ResponseTime = duration
	blocked := time.Duration(0)
	connecting := time.Duration(0)
	sending := time.Duration(0)
	waiting := time.Duration(0)
	receiving := time.Duration(0)
	tlsHandshaking := time.Duration(0)

	if !dnsStart.IsZero() {
		blocked = dnsStart.Sub(startTime)
	}
	if !connectStart.IsZero() && !connectEnd.IsZero() {
		connecting = connectEnd.Sub(connectStart)
	}
	if !tlsStart.IsZero() && !tlsEnd.IsZero() {
		tlsHandshaking = tlsEnd.Sub(tlsStart)
	}
	if !wroteRequest.IsZero() {
		if !connectEnd.IsZero() {
			sending = wroteRequest.Sub(connectEnd)
		} else if !dnsEnd.IsZero() {
			sending = wroteRequest.Sub(dnsEnd)
		} else {
			sending = wroteRequest.Sub(startTime)
		}
	}
	if !gotFirstByte.IsZero() && !wroteRequest.IsZero() {
		waiting = gotFirstByte.Sub(wroteRequest)
	}
	if !gotFirstByte.IsZero() {
		receiving = gotResponse.Sub(gotFirstByte)
	} else {
		// If we didn't get first byte timing, use total duration minus other phases
		receiving = duration - blocked - connecting - sending - tlsHandshaking
		if receiving < 0 {
			receiving = duration
		}
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()

		// Record failed request metrics
		tags := map[string]string{
			"method": req.Verb,
			"url":    renderedURL,
			"name":   req.Name,
			"status": "error",
		}
		r.MetricsCollector.RecordHTTPRequest(
			duration, blocked, connecting, sending, waiting, receiving, tlsHandshaking,
			true, int64(len(renderedBody)), 0, tags,
		)
		return result
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	responseBody := string(body)
	result.StatusCode = resp.StatusCode
	requestName := req.Name
	if requestName == "" {
		requestName = fmt.Sprintf("%s %s", req.Verb, renderedURL)
	}
	result.Name = requestName

	// Check if HTTP status code indicates success (2xx)
	isSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !isSuccess {
		result.Success = false
		result.Error = fmt.Sprintf("HTTP %d %s", resp.StatusCode, nethttp.StatusText(resp.StatusCode))
		fmt.Printf("%s [%s] [%s] URL: %s, Duration: %v, Body: %s\n", time.Now().Format("2006-01-02 15:04:05"), resp.Status, requestName, renderedURL, duration, responseBody)
	} else {
		result.Success = true
		fmt.Printf("%s [%s] [%s] URL: %s, Duration: %v\n", time.Now().Format("2006-01-02 15:04:05"), resp.Status, requestName, renderedURL, duration)
	}

	// Print verbose JSON output if enabled
	if r.Verbose {
		resultJSON, err := json.MarshalIndent(map[string]interface{}{
			"name":          result.Name,
			"verb":          result.Verb,
			"url":           result.URL,
			"timestamp":     result.Timestamp.Format(time.RFC3339),
			"virtualUserId": virtualUserId,
			"iterationId":   iterationID,
			"statusCode":    result.StatusCode,
			"responseTime":  duration.Milliseconds(),
			"success":       result.Success,
			"error":         result.Error,
			"responseBody":  responseBody,
		}, "", "  ")
		if err == nil {
			fmt.Printf("\n=== Request Result JSON ===\n%s\n===========================\n\n", string(resultJSON))
		}
	}

	// Record HTTP request metrics
	tags := map[string]string{
		"method": req.Verb,
		"url":    renderedURL,
		"name":   requestName,
		"status": fmt.Sprintf("%d", resp.StatusCode),
	}
	r.MetricsCollector.RecordHTTPRequest(
		duration, blocked, connecting, sending, waiting, receiving, tlsHandshaking,
		!isSuccess, int64(len(renderedBody)), int64(len(body)), tags,
	)

	// Execute post-request script if present
	if req.Script != "" {
		if err := te.ExecuteScript(req.Script, responseBody, virtualUserId, iterationID); err != nil {
			result.Success = false
			if assertErr, ok := err.(*template.AssertionError); ok {
				result.StatusCode = assertErr.StatusCode
				result.Error = assertErr.Message
			} else {
				result.Error = fmt.Sprintf("error executing script: %v", err)
			}
			return result
		}
	}

	// Collect check results from the template engine
	result.Checks = te.GetChecks()

	// Record check metrics
	for _, check := range result.Checks {
		checkTags := map[string]string{
			"check":  check.Name,
			"method": req.Verb,
			"name":   requestName,
		}
		r.MetricsCollector.RecordCheck(check.Success, checkTags)
	}

	return result
}

// executeForFunction executes a request and returns a Response object for JavaScript function calls
func (r *Runner) executeForFunction(req chttp.Request, te *template.Engine, virtualUserId, iterationID int) (*template.Response, error) {
	// Execute pre-request script if present
	if req.PreScript != "" {
		var scriptErr error
		var assertErr *template.AssertionError
		func() {
			defer func() {
				if r := recover(); r != nil {
					if ae, ok := r.(*template.AssertionError); ok {
						assertErr = ae
						return
					}
					panic(r) // Re-panic if it's not our assertion error
				}
			}()
			scriptErr = te.ExecuteScript(req.PreScript, "", virtualUserId, iterationID)
		}()

		if assertErr != nil {
			return &template.Response{
				StatusCode: assertErr.StatusCode,
				Headers:    make(map[string]string),
				Body: map[string]interface{}{
					"error": assertErr.Message,
				},
			}, assertErr
		}

		if scriptErr != nil {
			return &template.Response{
				StatusCode: 0,
				Headers:    make(map[string]string),
				Body: map[string]interface{}{
					"error": fmt.Sprintf("error executing pre-request script: %v", scriptErr),
				},
			}, fmt.Errorf("error executing pre-request script: %v", scriptErr)
		}
	}

	// Handle script-only requests (no HTTP verb/URL)
	if req.Verb == "" && req.URL == "" {
		// For script-only requests, just execute the post-request script and return success
		if req.Script != "" {
			var scriptErr error
			var assertErr *template.AssertionError
			func() {
				defer func() {
					if r := recover(); r != nil {
						if ae, ok := r.(*template.AssertionError); ok {
							assertErr = ae
							return
						}
						panic(r) // Re-panic if it's not our assertion error
					}
				}()
				scriptErr = te.ExecuteScript(req.Script, "", virtualUserId, iterationID)
			}()

			if assertErr != nil {
				return &template.Response{
					StatusCode: assertErr.StatusCode,
					Headers:    make(map[string]string),
					Body: map[string]interface{}{
						"error": assertErr.Message,
					},
				}, assertErr
			}

			if scriptErr != nil {
				return &template.Response{
					StatusCode: 0,
					Headers:    make(map[string]string),
					Body: map[string]interface{}{
						"error": fmt.Sprintf("error executing script: %v", scriptErr),
					},
				}, fmt.Errorf("error executing script: %v", scriptErr)
			}
		}
		return &template.Response{
			StatusCode: 200,
			Headers:    make(map[string]string),
			Body:       map[string]interface{}{"success": true},
		}, nil
	}

	// Render templates in URL
	renderedURL, err := te.RenderTemplate(req.URL)
	if err != nil {
		return &template.Response{
			StatusCode: 0,
			Headers:    make(map[string]string),
			Body: map[string]interface{}{
				"error": fmt.Sprintf("error rendering URL template: %v", err),
			},
		}, fmt.Errorf("error rendering URL template: %v", err)
	}

	// Render templates in headers
	renderedHeaders := make(map[string]string)
	for key, value := range req.Headers {
		renderedKey, err := te.RenderTemplate(key)
		if err != nil {
			return &template.Response{
				StatusCode: 0,
				Headers:    make(map[string]string),
				Body: map[string]interface{}{
					"error": fmt.Sprintf("error rendering header key template: %v", err),
				},
			}, fmt.Errorf("error rendering header key template: %v", err)
		}
		renderedValue, err := te.RenderTemplate(value)
		if err != nil {
			return &template.Response{
				StatusCode: 0,
				Headers:    make(map[string]string),
				Body: map[string]interface{}{
					"error": fmt.Sprintf("error rendering header value template: %v", err),
				},
			}, fmt.Errorf("error rendering header value template: %v", err)
		}
		renderedHeaders[renderedKey] = renderedValue
	}

	// Render templates in body
	renderedBody, err := te.RenderTemplate(req.Body)
	if err != nil {
		return &template.Response{
			StatusCode: 0,
			Headers:    make(map[string]string),
			Body: map[string]interface{}{
				"error": fmt.Sprintf("error rendering body template: %v", err),
			},
		}, fmt.Errorf("error rendering body template: %v", err)
	}

	// Create and execute HTTP request
	client := &nethttp.Client{}
	request, err := nethttp.NewRequest(req.Verb, renderedURL, bytes.NewBufferString(renderedBody))
	if err != nil {
		return &template.Response{
			StatusCode: 0,
			Headers:    make(map[string]string),
			Body: map[string]interface{}{
				"error": err.Error(),
			},
		}, err
	}

	for key, value := range renderedHeaders {
		request.Header.Set(key, value)
	}

	// Execute HTTP request
	resp, err := client.Do(request)
	if err != nil {
		return &template.Response{
			StatusCode: 0,
			Headers:    make(map[string]string),
			Body: map[string]interface{}{
				"error": err.Error(),
			},
		}, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &template.Response{
			StatusCode: resp.StatusCode,
			Headers:    make(map[string]string),
			Body: map[string]interface{}{
				"error": err.Error(),
			},
		}, err
	}

	// Convert response headers to map[string]string
	responseHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			responseHeaders[key] = values[0] // Take the first value if multiple
		}
	}

	responseBody := string(body)

	// Parse response body as JSON if possible, otherwise use raw string
	var parsedBody interface{}
	if responseBody != "" {
		if err := json.Unmarshal([]byte(responseBody), &parsedBody); err != nil {
			// If JSON parsing fails, use raw string
			parsedBody = responseBody
		}
	} else {
		parsedBody = ""
	}

	response := &template.Response{
		StatusCode: resp.StatusCode,
		Headers:    responseHeaders,
		Body:       parsedBody,
	}

	// Execute post-request script if present
	if req.Script != "" {
		var scriptErr error
		var assertErr *template.AssertionError
		func() {
			defer func() {
				if r := recover(); r != nil {
					if ae, ok := r.(*template.AssertionError); ok {
						assertErr = ae
						return
					}
					panic(r) // Re-panic if it's not our assertion error
				}
			}()
			scriptErr = te.ExecuteScript(req.Script, responseBody, virtualUserId, iterationID)
		}()

		if assertErr != nil {
			// Return the response with assertion error details
			response.StatusCode = assertErr.StatusCode
			response.Body = map[string]interface{}{
				"error": assertErr.Message,
			}
			return response, assertErr
		}

		if scriptErr != nil {
			return response, fmt.Errorf("error executing script: %v", scriptErr)
		}
	}

	return response, nil
}
