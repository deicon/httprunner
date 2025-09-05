package reporting

import (
	"sort"
	"time"
)

// Collector gathers request results and generates reports
type Collector struct {
	results   []RequestResult
	startTime time.Time
}

// NewCollector creates a new report collector
func NewCollector() *Collector {
	return &Collector{
		results:   make([]RequestResult, 0),
		startTime: time.Now(),
	}
}

// AddResult adds a request result to the collector
func (c *Collector) AddResult(result RequestResult) {
	c.results = append(c.results, result)
}

// GenerateReport creates a comprehensive report from collected results
func (c *Collector) GenerateReport() *Report {
	if len(c.results) == 0 {
		return &Report{
			StartTime: c.startTime,
			EndTime:   time.Now(),
		}
	}

	report := &Report{
		TotalRequests:            len(c.results),
		ResponseTimeDistribution: make(map[string]int),
		ErrorBreakdown:          make(map[string]int),
		RequestDetails:          c.results,
		StartTime:               c.startTime,
		EndTime:                 time.Now(),
	}

	var totalResponseTime time.Duration
	responseTimes := make([]time.Duration, len(c.results))

	for i, result := range c.results {
		responseTimes[i] = result.ResponseTime
		totalResponseTime += result.ResponseTime

		if result.Success {
			report.SuccessfulRequests++
		} else {
			report.FailedRequests++
			if result.Error != "" {
				report.ErrorBreakdown[result.Error]++
			}
		}

		// Categorize response times
		ms := result.ResponseTime.Milliseconds()
		if ms < 100 {
			report.ResponseTimeDistribution["<100ms"]++
		} else if ms < 500 {
			report.ResponseTimeDistribution["100-500ms"]++
		} else if ms < 1000 {
			report.ResponseTimeDistribution["500ms-1s"]++
		} else {
			report.ResponseTimeDistribution[">1s"]++
		}
	}

	// Calculate average response time
	if len(c.results) > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(len(c.results))
	}

	// Find min and max response times
	sort.Slice(responseTimes, func(i, j int) bool {
		return responseTimes[i] < responseTimes[j]
	})
	report.MinResponseTime = responseTimes[0]
	report.MaxResponseTime = responseTimes[len(responseTimes)-1]

	return report
}