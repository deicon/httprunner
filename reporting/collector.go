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
		ErrorBreakdown:           make(map[string]int),
		RequestDetails:           c.results,
		StartTime:                c.startTime,
		EndTime:                  time.Now(),
		CheckSummaries:           make(map[string]CheckSummary),
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

		// Process checks
		for _, check := range result.Checks {
			report.TotalChecks++
			if check.Success {
				report.SuccessfulChecks++
			} else {
				report.FailedChecks++
			}

			// Update check summary
			summary, exists := report.CheckSummaries[check.Name]
			if !exists {
				summary = CheckSummary{
					Name:            check.Name,
					FailureMessages: make([]string, 0),
				}
			}

			summary.TotalRuns++
			if check.Success {
				summary.SuccessfulRuns++
			} else {
				summary.FailedRuns++
				if check.FailureMessage != "" && !contains(summary.FailureMessages, check.FailureMessage) {
					summary.FailureMessages = append(summary.FailureMessages, check.FailureMessage)
				}
			}

			report.CheckSummaries[check.Name] = summary
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

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GenerateHierarchicalReport creates a hierarchical report with goroutine and iteration breakdown
func (c *Collector) GenerateHierarchicalReport() *HierarchicalReport {
	// First generate the summary report
	summaryReport := c.GenerateReport()

	// Group results by virtual user and iteration
	goroutineMap := make(map[int]map[int][]RequestResult)

	for _, result := range c.results {
		if goroutineMap[result.VirtualUserID] == nil {
			goroutineMap[result.VirtualUserID] = make(map[int][]RequestResult)
		}
		goroutineMap[result.VirtualUserID][result.IterationID] = append(
			goroutineMap[result.VirtualUserID][result.IterationID], result)
	}

	hierarchical := &HierarchicalReport{
		Summary:            *summaryReport,
		VirtualUserReports: make([]GoroutineReport, 0, len(goroutineMap)),
		TotalVirtualUsers:  len(goroutineMap),
	}

	// Generate goroutine reports
	for goroutineID, iterations := range goroutineMap {
		goroutineReport := c.generateGoroutineReport(goroutineID, iterations)
		hierarchical.VirtualUserReports = append(hierarchical.VirtualUserReports, goroutineReport)

		// Count successful goroutines (goroutines with at least one successful iteration)
		if goroutineReport.SuccessfulIterations > 0 {
			hierarchical.SuccessfulVirtualUsers++
		} else {
			hierarchical.FailedVirtualUsers++
		}
	}

	return hierarchical
}

// generateGoroutineReport creates a report for a single goroutine
func (c *Collector) generateGoroutineReport(goroutineID int, iterations map[int][]RequestResult) GoroutineReport {
	report := GoroutineReport{
		GoroutineID:     goroutineID,
		Iterations:      make([]IterationReport, 0, len(iterations)),
		TotalIterations: len(iterations),
		StartTime:       time.Now(),
		EndTime:         time.Time{},
	}

	var totalResponseTime time.Duration
	var totalRequests int

	for iterationID, results := range iterations {
		iterationReport := c.generateIterationReport(iterationID, results)
		report.Iterations = append(report.Iterations, iterationReport)

		// Aggregate goroutine stats from iterations
		report.TotalRequests += iterationReport.TotalRequests
		report.SuccessfulRequests += iterationReport.SuccessfulRequests
		report.FailedRequests += iterationReport.FailedRequests
		totalRequests += iterationReport.TotalRequests

		// Calculate response time for each request in iteration
		for _, result := range results {
			totalResponseTime += result.ResponseTime
		}

		// Count successful iterations (iterations with all requests successful)
		if iterationReport.FailedRequests == 0 && iterationReport.TotalRequests > 0 {
			report.SuccessfulIterations++
		} else {
			report.FailedIterations++
		}

		// Track time bounds
		if iterationReport.StartTime.Before(report.StartTime) || report.StartTime.IsZero() {
			report.StartTime = iterationReport.StartTime
		}
		if iterationReport.EndTime.After(report.EndTime) {
			report.EndTime = iterationReport.EndTime
		}
	}

	// Calculate averages
	if totalRequests > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(totalRequests)
	}
	report.TotalDuration = report.EndTime.Sub(report.StartTime)

	return report
}

// generateIterationReport creates a report for a single iteration
func (c *Collector) generateIterationReport(iterationID int, results []RequestResult) IterationReport {
	report := IterationReport{
		IterationID:    iterationID,
		RequestResults: results,
		TotalRequests:  len(results),
	}

	if len(results) == 0 {
		return report
	}

	var totalResponseTime time.Duration
	report.StartTime = results[0].Timestamp
	report.EndTime = results[0].Timestamp

	for _, result := range results {
		totalResponseTime += result.ResponseTime

		if result.Success {
			report.SuccessfulRequests++
		} else {
			report.FailedRequests++
		}

		// Track time bounds
		if result.Timestamp.Before(report.StartTime) {
			report.StartTime = result.Timestamp
		}
		if result.Timestamp.After(report.EndTime) {
			report.EndTime = result.Timestamp
		}
	}

	// Calculate averages
	if len(results) > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(len(results))
	}
	report.TotalDuration = report.EndTime.Sub(report.StartTime)

	return report
}
