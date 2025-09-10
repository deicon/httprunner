package runner

import (
	"bytes"
	"context"
	"crypto/tls"
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
	Delay              int
	Requests           []chttp.Request
	envFile            string
	StreamingCollector *streaming.StreamingCollector
	OutputDir          string
	MetricsCollector   *metrics.MetricsCollector
}

// NewRunner creates a new Runner with file streaming for memory efficiency
func NewRunner(concurrency, iterations, delay int, requests []chttp.Request, outputDir string) (*Runner, error) {
	streamingCollector, err := streaming.NewStreamingCollector(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming collector: %v", err)
	}

	metricsCollector := metrics.NewMetricsCollector()

	return &Runner{
		Concurrency:        concurrency,
		Iterations:         iterations,
		Delay:              delay,
		Requests:           requests,
		StreamingCollector: streamingCollector,
		OutputDir:          outputDir,
		MetricsCollector:   metricsCollector,
	}, nil
}

// NewRunnerWithEnvFile creates a new Runner with streaming and env file support
func NewRunnerWithEnvFile(concurrency, iterations, delay int, requests []chttp.Request, envFile, outputDir string) (*Runner, error) {
	streamingCollector, err := streaming.NewStreamingCollector(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming collector: %v", err)
	}

	metricsCollector := metrics.NewMetricsCollector()

	return &Runner{
		Concurrency:        concurrency,
		Iterations:         iterations,
		Delay:              delay,
		Requests:           requests,
		envFile:            envFile,
		StreamingCollector: streamingCollector,
		OutputDir:          outputDir,
		MetricsCollector:   metricsCollector,
	}, nil
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

	wg.Add(r.Concurrency)

	// Update VU metrics
	r.MetricsCollector.UpdateVirtualUsers(r.Concurrency, r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				iterationStart := time.Now()
				templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)

				// Pass metrics collector to template engine for check access
				templateEngine.SetMetricsCollector(r.MetricsCollector)

				for _, req := range r.Requests {
					result := r.execute(req, templateEngine, workerID, j)
					resultChan <- result
					if !result.Success {
						fmt.Printf("[Worker %d] Error: %v - Stopping iteration %d\n", workerID, result.Error, j+1)
						break
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
				}

				// Record iteration metrics
				iterationDuration := time.Since(iterationStart)
				tags := map[string]string{
					"vu":        fmt.Sprintf("%d", workerID),
					"iteration": fmt.Sprintf("%d", j),
				}
				r.MetricsCollector.RecordIteration(iterationDuration, tags)
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
		if err := te.ExecuteScript(req.PreScript, ""); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("error executing pre-request script: %v", err)
			return result
		}
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
		if err := te.ExecuteScript(req.Script, responseBody); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("error executing script: %v", err)
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
