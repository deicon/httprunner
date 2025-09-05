package reporting

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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

// GenerateReport creates a comprehensive report by streaming from the results file
func (fr *FileReporter) GenerateReport(startTime time.Time) (*Report, error) {
	return fr.generateReportStreaming(startTime)
}

// generateReportStreaming processes the results file line by line to avoid memory issues
func (fr *FileReporter) generateReportStreaming(startTime time.Time) (*Report, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open results file: %v", err)
	}
	defer file.Close()

	report := &Report{
		ResponseTimeDistribution: make(map[string]int),
		ErrorBreakdown:           make(map[string]int),
		StartTime:                startTime,
		EndTime:                  time.Now(),
		MinResponseTime:          time.Hour, // Initialize to high value
		MaxResponseTime:          0,
	}

	scanner := bufio.NewScanner(file)
	var totalResponseTime time.Duration

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var result RequestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %v", err)
		}

		// Update counters
		report.TotalRequests++
		totalResponseTime += result.ResponseTime

		if result.Success {
			report.SuccessfulRequests++
		} else {
			report.FailedRequests++
			if result.Error != "" {
				report.ErrorBreakdown[result.Error]++
			}
		}

		// Update min/max response times
		if result.ResponseTime < report.MinResponseTime {
			report.MinResponseTime = result.ResponseTime
		}
		if result.ResponseTime > report.MaxResponseTime {
			report.MaxResponseTime = result.ResponseTime
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

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	// Calculate average response time
	if report.TotalRequests > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(report.TotalRequests)
	} else {
		// Reset min response time if no results
		report.MinResponseTime = 0
	}

	return report, nil
}

// GenerateHierarchicalReport creates a hierarchical report by streaming from the results file
func (fr *FileReporter) GenerateHierarchicalReport(startTime time.Time) (*HierarchicalReport, error) {
	return fr.generateHierarchicalReportStreaming(startTime)
}

// generateHierarchicalReportStreaming processes results in passes to minimize memory usage
func (fr *FileReporter) generateHierarchicalReportStreaming(startTime time.Time) (*HierarchicalReport, error) {
	// Generate summary report using streaming
	summaryReport, err := fr.generateReportStreaming(startTime)
	if err != nil {
		return nil, err
	}

	// First pass: collect goroutine/iteration metadata without storing full results
	goroutineStats, err := fr.collectGoroutineMetadata()
	if err != nil {
		return nil, err
	}

	hierarchical := &HierarchicalReport{
		Summary:         *summaryReport,
		Goroutines:      make([]GoroutineReport, 0, len(goroutineStats)),
		TotalGoroutines: len(goroutineStats),
	}

	// Generate goroutine reports using streaming approach
	for goroutineID, metadata := range goroutineStats {
		goroutineReport, err := fr.generateGoroutineReportStreaming(goroutineID, metadata)
		if err != nil {
			return nil, err
		}
		hierarchical.Goroutines = append(hierarchical.Goroutines, goroutineReport)

		// Count successful goroutines
		if goroutineReport.SuccessfulIterations > 0 {
			hierarchical.SuccessfulGoroutines++
		} else {
			hierarchical.FailedGoroutines++
		}
	}

	return hierarchical, nil
}

// goroutineIterationMetadata holds minimal metadata for streaming processing
type goroutineIterationMetadata struct {
	iterationIDs map[int]bool
}

// collectGoroutineMetadata makes a first pass to collect goroutine/iteration structure
func (fr *FileReporter) collectGoroutineMetadata() (map[int]*goroutineIterationMetadata, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open results file: %v", err)
	}
	defer file.Close()

	goroutineStats := make(map[int]*goroutineIterationMetadata)
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

		if goroutineStats[result.GoroutineID] == nil {
			goroutineStats[result.GoroutineID] = &goroutineIterationMetadata{
				iterationIDs: make(map[int]bool),
			}
		}
		goroutineStats[result.GoroutineID].iterationIDs[result.IterationID] = true
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return goroutineStats, nil
}

// generateGoroutineReportStreaming generates a goroutine report by streaming through the file
func (fr *FileReporter) generateGoroutineReportStreaming(goroutineID int, metadata *goroutineIterationMetadata) (GoroutineReport, error) {
	report := GoroutineReport{
		GoroutineID:     goroutineID,
		Iterations:      make([]IterationReport, 0, len(metadata.iterationIDs)),
		TotalIterations: len(metadata.iterationIDs),
		StartTime:       time.Now(),
		EndTime:         time.Time{},
	}

	// Process each iteration for this goroutine
	for iterationID := range metadata.iterationIDs {
		iterationReport, err := fr.generateIterationReportStreaming(goroutineID, iterationID)
		if err != nil {
			return report, err
		}
		report.Iterations = append(report.Iterations, iterationReport)

		// Aggregate stats
		report.TotalRequests += iterationReport.TotalRequests
		report.SuccessfulRequests += iterationReport.SuccessfulRequests
		report.FailedRequests += iterationReport.FailedRequests

		// Count successful iterations
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
	if report.TotalRequests > 0 {
		// We need to calculate total response time by streaming through matching results
		totalResponseTime, err := fr.calculateTotalResponseTime(goroutineID, -1)
		if err != nil {
			return report, err
		}
		report.AverageResponseTime = totalResponseTime / time.Duration(report.TotalRequests)
	}
	report.TotalDuration = report.EndTime.Sub(report.StartTime)

	return report, nil
}

// generateIterationReportStreaming generates an iteration report by streaming through the file
func (fr *FileReporter) generateIterationReportStreaming(goroutineID, iterationID int) (IterationReport, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return IterationReport{}, fmt.Errorf("failed to open results file: %v", err)
	}
	defer file.Close()

	report := IterationReport{
		IterationID:    iterationID,
		RequestResults: nil, // Don't store individual results to save memory
		TotalRequests:  0,
		StartTime:      time.Now(),
		EndTime:        time.Time{},
	}

	scanner := bufio.NewScanner(file)
	var totalResponseTime time.Duration
	firstResult := true

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var result RequestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return report, fmt.Errorf("failed to unmarshal result: %v", err)
		}

		// Skip results not for this goroutine/iteration
		if result.GoroutineID != goroutineID || result.IterationID != iterationID {
			continue
		}

		report.TotalRequests++
		totalResponseTime += result.ResponseTime

		if result.Success {
			report.SuccessfulRequests++
		} else {
			report.FailedRequests++
		}

		// Track time bounds
		if firstResult {
			report.StartTime = result.Timestamp
			report.EndTime = result.Timestamp
			firstResult = false
		} else {
			if result.Timestamp.Before(report.StartTime) {
				report.StartTime = result.Timestamp
			}
			if result.Timestamp.After(report.EndTime) {
				report.EndTime = result.Timestamp
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return report, fmt.Errorf("error reading file: %v", err)
	}

	// Calculate averages
	if report.TotalRequests > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(report.TotalRequests)
	}
	report.TotalDuration = report.EndTime.Sub(report.StartTime)

	return report, nil
}

// calculateTotalResponseTime calculates total response time for a goroutine/iteration filter
func (fr *FileReporter) calculateTotalResponseTime(goroutineID, iterationID int) (time.Duration, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open results file: %v", err)
	}
	defer file.Close()

	var totalResponseTime time.Duration
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var result RequestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return 0, fmt.Errorf("failed to unmarshal result: %v", err)
		}

		// Filter by goroutine and optionally by iteration (-1 means all iterations)
		if result.GoroutineID == goroutineID && (iterationID == -1 || result.IterationID == iterationID) {
			totalResponseTime += result.ResponseTime
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading file: %v", err)
	}

	return totalResponseTime, nil
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
