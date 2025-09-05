package reporting

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// FileReporter generates reports from streamed result files
type FileReporter struct {
	resultsFilePath string
}

// NewFileReporter creates a new file-based reporter
func NewFileReporter(resultsFilePath string) *FileReporter {
	return &FileReporter{
		resultsFilePath: resultsFilePath,
	}
}

// GenerateReport creates a comprehensive report by reading from the results file
func (fr *FileReporter) GenerateReport(startTime time.Time) (*Report, error) {
	results, err := fr.readResults()
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &Report{
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	report := &Report{
		TotalRequests:            len(results),
		ResponseTimeDistribution: make(map[string]int),
		ErrorBreakdown:          make(map[string]int),
		StartTime:               startTime,
		EndTime:                 time.Now(),
	}

	// Only include RequestDetails for summary reports to avoid memory issues
	// For detailed analysis, users should analyze the raw file directly
	
	var totalResponseTime time.Duration
	responseTimes := make([]time.Duration, len(results))

	for i, result := range results {
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
	if len(results) > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(len(results))
	}

	// Find min and max response times
	sort.Slice(responseTimes, func(i, j int) bool {
		return responseTimes[i] < responseTimes[j]
	})
	report.MinResponseTime = responseTimes[0]
	report.MaxResponseTime = responseTimes[len(responseTimes)-1]

	return report, nil
}

// GenerateHierarchicalReport creates a hierarchical report by reading from the results file
func (fr *FileReporter) GenerateHierarchicalReport(startTime time.Time) (*HierarchicalReport, error) {
	results, err := fr.readResults()
	if err != nil {
		return nil, err
	}

	// Generate summary report
	summaryReport, err := fr.GenerateReport(startTime)
	if err != nil {
		return nil, err
	}

	// Group results by goroutine and iteration
	goroutineMap := make(map[int]map[int][]RequestResult)
	
	for _, result := range results {
		if goroutineMap[result.GoroutineID] == nil {
			goroutineMap[result.GoroutineID] = make(map[int][]RequestResult)
		}
		goroutineMap[result.GoroutineID][result.IterationID] = append(
			goroutineMap[result.GoroutineID][result.IterationID], result)
	}
	
	hierarchical := &HierarchicalReport{
		Summary:         *summaryReport,
		Goroutines:      make([]GoroutineReport, 0, len(goroutineMap)),
		TotalGoroutines: len(goroutineMap),
	}
	
	// Generate goroutine reports
	for goroutineID, iterations := range goroutineMap {
		goroutineReport := fr.generateGoroutineReport(goroutineID, iterations)
		hierarchical.Goroutines = append(hierarchical.Goroutines, goroutineReport)
		
		// Count successful goroutines (goroutines with at least one successful iteration)
		if goroutineReport.SuccessfulIterations > 0 {
			hierarchical.SuccessfulGoroutines++
		} else {
			hierarchical.FailedGoroutines++
		}
	}
	
	return hierarchical, nil
}

// readResults reads all results from the JSONL file
func (fr *FileReporter) readResults() ([]RequestResult, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open results file: %v", err)
	}
	defer file.Close()

	var results []RequestResult
	scanner := bufio.NewScanner(file)
	
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		
		var result RequestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %v", err)
		}
		
		results = append(results, result)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}
	
	return results, nil
}

// generateGoroutineReport creates a report for a single goroutine from file data
func (fr *FileReporter) generateGoroutineReport(goroutineID int, iterations map[int][]RequestResult) GoroutineReport {
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
		iterationReport := fr.generateIterationReport(iterationID, results)
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

// generateIterationReport creates a report for a single iteration from file data
func (fr *FileReporter) generateIterationReport(iterationID int, results []RequestResult) IterationReport {
	report := IterationReport{
		IterationID:    iterationID,
		RequestResults: nil, // Don't store individual results to save memory
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