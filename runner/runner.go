package runner

import (
	"bytes"
	"fmt"
	"io"
	nethttp "net/http"
	"sync"
	"time"

	chttp "github.com/deicon/httprunner/http"
	"github.com/deicon/httprunner/template"
)

// Runner executes HTTP requests
type Runner struct {
	Concurrency int
	Iterations  int
	Delay       int
	Requests    []chttp.Request
	envFile     string
}

// NewRunner creates a new Runner
func NewRunner(concurrency, iterations, delay int, requests []chttp.Request) *Runner {
	return &Runner{
		Concurrency: concurrency,
		Iterations:  iterations,
		Delay:       delay,
		Requests:    requests,
	}
}

// NewRunnerWithEnvFile creates a new Runner with env file support
func NewRunnerWithEnvFile(concurrency, iterations, delay int, requests []chttp.Request, envFile string) (*Runner, error) {

	return &Runner{
		Concurrency: concurrency,
		Iterations:  iterations,
		Delay:       delay,
		Requests:    requests,
		envFile:     envFile,
	}, nil
}

// Run executes the requests
func (r *Runner) Run() {
	var wg sync.WaitGroup
	wg.Add(r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				templateEngine, _ := template.NewTemplateEngineWithEnvFile(r.envFile)
				for _, req := range r.Requests {
					if err := r.execute(req, templateEngine); err != nil {
						fmt.Printf("[Worker %d] Error: %v\n", workerID, err)
					}
					time.Sleep(time.Duration(r.Delay) * time.Millisecond)
				}
			}
		}(i)
	}

	wg.Wait()
}

func (r *Runner) execute(req chttp.Request, te *template.Engine) error {
	// Execute pre-request script if present
	if req.PreScript != "" {
		if err := te.ExecuteScript(req.PreScript, ""); err != nil {
			return fmt.Errorf("error executing pre-request script: %v", err)
		}
	}

	// Render templates in URL
	renderedURL, err := te.RenderTemplate(req.URL)
	if err != nil {
		return fmt.Errorf("error rendering URL template: %v", err)
	}

	// Render templates in headers
	renderedHeaders := make(map[string]string)
	for key, value := range req.Headers {
		renderedKey, err := te.RenderTemplate(key)
		if err != nil {
			return fmt.Errorf("error rendering header key template: %v", err)
		}
		renderedValue, err := te.RenderTemplate(value)
		if err != nil {
			return fmt.Errorf("error rendering header value template: %v", err)
		}
		renderedHeaders[renderedKey] = renderedValue
	}

	// Render templates in body
	renderedBody, err := te.RenderTemplate(req.Body)
	if err != nil {
		return fmt.Errorf("error rendering body template: %v", err)
	}

	// Create and execute HTTP request
	client := &nethttp.Client{}
	request, err := nethttp.NewRequest(req.Verb, renderedURL, bytes.NewBufferString(renderedBody))
	if err != nil {
		return err
	}

	for key, value := range renderedHeaders {
		request.Header.Set(key, value)
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			return
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	responseBody := string(body)
	requestName := req.Name
	if requestName == "" {
		requestName = fmt.Sprintf("%s %s", req.Verb, renderedURL)
	}

	fmt.Printf("[%s] Status: %s, Body: %s\n", requestName, resp.Status, responseBody)

	// Execute post-request script if present
	if req.Script != "" {
		if err := te.ExecuteScript(req.Script, responseBody); err != nil {
			return fmt.Errorf("error executing script: %v", err)
		}
	}

	return nil
}
