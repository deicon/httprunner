package runner

import (
	"bytes"
	chttp "curlrunner/http"
	"fmt"
	"io/ioutil"
	nethttp "net/http"
	"sync"
	"time"
)

// Runner executes HTTP requests
type Runner struct {
	Concurrency int
	Iterations  int
	Delay       int
	Requests    []chttp.Request
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

// Run executes the requests
func (r *Runner) Run() {
	var wg sync.WaitGroup
	wg.Add(r.Concurrency)

	for i := 0; i < r.Concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < r.Iterations; j++ {
				for _, req := range r.Requests {
					if err := r.execute(req); err != nil {
						fmt.Printf("[Worker %d] Error: %v\n", workerID, err)
					}
				}
				time.Sleep(time.Duration(r.Delay) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
}

func (r *Runner) execute(req chttp.Request) error {
	client := &nethttp.Client{}
	request, err := nethttp.NewRequest(req.Verb, req.URL, bytes.NewBufferString(req.Body))
	if err != nil {
		return err
	}

	for key, value := range req.Headers {
		request.Header.Set(key, value)
	}

	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("Status: %s, Body: %s\n", resp.Status, string(body))
	return nil
}
