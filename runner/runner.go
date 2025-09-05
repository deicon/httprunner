package runner

import (
	"bytes"
	"fmt"
	"io"
	nethttp "net/http"
	"sync"
	"time"

	chttp "github.com/deicon/httprunner/http"
	"github.com/deicon/httprunner/reporting"
	"github.com/deicon/httprunner/template"
)

// Runner executes HTTP requests
type Runner struct {
	Concurrency       int
	Iterations        int
	Delay             int
	Requests          []chttp.Request
	envFile           string
	Collector         *reporting.Collector
	StreamingCollector *reporting.StreamingCollector
	OutputDir         string
}

// NewRunner creates a new Runner with in-memory collection (legacy)
func NewRunner(concurrency, iterations, delay int, requests []chttp.Request) *Runner {
	return &Runner{
		Concurrency: concurrency,
		Iterations:  iterations,
		Delay:       delay,
		Requests:    requests,
		Collector:   reporting.NewCollector(),
	}
}

// NewStreamingRunner creates a new Runner with file streaming for memory efficiency
func NewStreamingRunner(concurrency, iterations, delay int, requests []chttp.Request, outputDir string) (*Runner, error) {
	streamingCollector, err := reporting.NewStreamingCollector(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming collector: %v", err)
	}

	return &Runner{
		Concurrency:       concurrency,
		Iterations:        iterations,
		Delay:             delay,
		Requests:          requests,
		StreamingCollector: streamingCollector,
		OutputDir:         outputDir,
	}, nil
}

// NewRunnerWithEnvFile creates a new Runner with env file support (legacy)
func NewRunnerWithEnvFile(concurrency, iterations, delay int, requests []chttp.Request, envFile string) (*Runner, error) {
	return &Runner{
		Concurrency: concurrency,
		Iterations:  iterations,
		Delay:       delay,
		Requests:    requests,
		envFile:     envFile,
		Collector:   reporting.NewCollector(),
	}, nil
}

// NewStreamingRunnerWithEnvFile creates a new Runner with streaming and env file support
func NewStreamingRunnerWithEnvFile(concurrency, iterations, delay int, requests []chttp.Request, envFile, outputDir string) (*Runner, error) {
	streamingCollector, err := reporting.NewStreamingCollector(outputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create streaming collector: %v", err)
	}

	return &Runner{
		Concurrency:       concurrency,
		Iterations:        iterations,
		Delay:             delay,
		Requests:          requests,
		envFile:           envFile,
		StreamingCollector: streamingCollector,
		OutputDir:         outputDir,
	}, nil
}

// Run executes the requests and returns a report
func (r *Runner) Run() *reporting.Report {
	var wg sync.WaitGroup
	resultChan := make(chan reporting.RequestResult, r.Concurrency*r.Iterations*len(r.Requests))

	wg.Add(r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
				for _, req := range r.Requests {
					result := r.execute(req, templateEngine, workerID, j)
					resultChan <- result
					if !result.Success {
						fmt.Printf("[Worker %d] Error: %v - Stopping iteration %d\n", workerID, result.Error, j+1)
						return
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
				}
			}
		}(i)
	}

	// Collect results in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results
	for result := range resultChan {
		r.Collector.AddResult(result)
	}

	return r.Collector.GenerateReport()
}

// RunStreaming executes requests with file streaming to reduce memory usage
func (r *Runner) RunStreaming() (*reporting.Report, error) {
	if r.StreamingCollector == nil {
		return nil, fmt.Errorf("streaming collector not initialized, use NewStreamingRunner")
	}

	startTime := r.StreamingCollector.GetStartTime()
	
	var wg sync.WaitGroup
	resultChan := make(chan reporting.RequestResult, r.Concurrency*r.Iterations*len(r.Requests))
	
	wg.Add(r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
				for _, req := range r.Requests {
					result := r.execute(req, templateEngine, workerID, j)
					resultChan <- result
					if !result.Success {
						fmt.Printf("[Worker %d] Error: %v - Stopping iteration %d\n", workerID, result.Error, j+1)
						return
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
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
			return nil, fmt.Errorf("failed to stream result: %v", err)
		}
	}

	// Close streaming collector
	if err := r.StreamingCollector.Close(); err != nil {
		return nil, fmt.Errorf("failed to close streaming collector: %v", err)
	}

	// Generate report from file
	fileReporter := reporting.NewFileReporter(r.StreamingCollector.GetResultsFilePath())
	return fileReporter.GenerateReport(startTime)
}

// RunHierarchical executes the requests and returns a hierarchical report
func (r *Runner) RunHierarchical() *reporting.HierarchicalReport {
	var wg sync.WaitGroup
	resultChan := make(chan reporting.RequestResult, r.Concurrency*r.Iterations*len(r.Requests))

	wg.Add(r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
				for _, req := range r.Requests {
					result := r.execute(req, templateEngine, workerID, j)
					resultChan <- result
					if !result.Success {
						fmt.Printf("[Worker %d] Error: %v - Stopping iteration %d\n", workerID, result.Error, j+1)
						return
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
				}
			}
		}(i)
	}

	// Collect results in a separate goroutine
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect all results
	for result := range resultChan {
		r.Collector.AddResult(result)
	}

	return r.Collector.GenerateHierarchicalReport()
}

// RunHierarchicalStreaming executes requests with file streaming and returns hierarchical report
func (r *Runner) RunHierarchicalStreaming() (*reporting.HierarchicalReport, error) {
	if r.StreamingCollector == nil {
		return nil, fmt.Errorf("streaming collector not initialized, use NewStreamingRunner")
	}

	startTime := r.StreamingCollector.GetStartTime()
	
	var wg sync.WaitGroup
	resultChan := make(chan reporting.RequestResult, r.Concurrency*r.Iterations*len(r.Requests))
	
	wg.Add(r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
				for _, req := range r.Requests {
					result := r.execute(req, templateEngine, workerID, j)
					resultChan <- result
					if !result.Success {
						fmt.Printf("[Worker %d] Error: %v - Stopping iteration %d\n", workerID, result.Error, j+1)
						return
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
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
			return nil, fmt.Errorf("failed to stream result: %v", err)
		}
	}

	// Close streaming collector
	if err := r.StreamingCollector.Close(); err != nil {
		return nil, fmt.Errorf("failed to close streaming collector: %v", err)
	}

	// Generate hierarchical report from file
	fileReporter := reporting.NewFileReporter(r.StreamingCollector.GetResultsFilePath())
	return fileReporter.GenerateHierarchicalReport(startTime)
}

func (r *Runner) execute(req chttp.Request, te *template.Engine, goroutineID, iterationID int) reporting.RequestResult {
	result := reporting.RequestResult{
		Name:        req.Name,
		Verb:        req.Verb,
		URL:         req.URL,
		Timestamp:   time.Now(),
		GoroutineID: goroutineID,
		IterationID: iterationID,
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

	start := time.Now()
	resp, err := client.Do(request)
	duration := time.Since(start)
	result.ResponseTime = duration

	if err != nil {
		result.Success = false
		result.Error = err.Error()
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Success = false
		result.Error = fmt.Sprintf("HTTP %d %s", resp.StatusCode, nethttp.StatusText(resp.StatusCode))
		fmt.Printf("%s [%s] [%s] URL: %s, Duration: %v, Body: %s\n", time.Now().Format("2006-01-02 15:04:05"), resp.Status, requestName, renderedURL, duration, responseBody)
	} else {
		result.Success = true
		fmt.Printf("%s [%s] [%s] URL: %s, Duration: %v, Body: %s\n", time.Now().Format("2006-01-02 15:04:05"), resp.Status, requestName, renderedURL, duration, responseBody)
	}

	// Execute post-request script if present
	if req.Script != "" {
		if err := te.ExecuteScript(req.Script, responseBody); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("error executing script: %v", err)
			return result
		}
	}

	return result
}
