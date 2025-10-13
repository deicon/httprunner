package reporting

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/deicon/httprunner/metrics"
	"github.com/deicon/httprunner/reporting/types"
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
func (fr *FileReporter) GenerateReport(startTime time.Time) (*types.Report, error) {
	return fr.generateReportStreaming(startTime)
}

// generateReportStreaming processes the results file line by line to avoid memory issues
func (fr *FileReporter) generateReportStreaming(startTime time.Time) (*types.Report, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open results file: %v", err)
	}
	defer func() {
		_ = file.Close() // Ignore close error for read-only operations
	}()

	report := &types.Report{
		ResponseTimeDistribution: make(map[string]int),
		ErrorBreakdown:           make(map[string]int),
		CheckSummaries:           make(map[string]types.CheckSummary),
		RequestDetails:           make([]types.RequestResult, 0),
		TopLongestRequests:       make([]types.RequestResult, 0),
		StartTime:                startTime,
		EndTime:                  time.Time{},
		MinResponseTime:          time.Hour, // Initialize to high value
		MaxResponseTime:          0,
		MetricsSummaries:         make(map[string]metrics.MetricSummary),
	}

	scanner := bufio.NewScanner(file)
	var totalResponseTime time.Duration
	// Track first/last timestamps and durations to compute offline metrics
	var firstTimestamp time.Time
	var lastTimestamp time.Time
	durationsMs := make([]float64, 0, 1024)
	// Track unique virtual users
	vus := make(map[int]struct{})

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var result types.RequestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result: %v", err)
		}

		// Update counters
		report.TotalRequests++
		totalResponseTime += result.ResponseTime

		// Track VU set
		vus[result.VirtualUserID] = struct{}{}

		if result.Success {
			report.SuccessfulRequests++
		} else {
			report.FailedRequests++
			if result.Error != "" {
				report.ErrorBreakdown[result.Error]++
			}
		}

		// Track timestamps for runtime
		if firstTimestamp.IsZero() || result.Timestamp.Before(firstTimestamp) {
			firstTimestamp = result.Timestamp
		}
		if lastTimestamp.IsZero() || result.Timestamp.After(lastTimestamp) {
			lastTimestamp = result.Timestamp
		}

		// Collect durations (ms) for trend metrics
		durationsMs = append(durationsMs, float64(result.ResponseTime.Milliseconds()))

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

		// Process checks
		for _, check := range result.Checks {
			report.TotalChecks++
			if check.Success {
				report.SuccessfulChecks++
			} else {
				report.FailedChecks++
			}

			// Update check summary
			checkSummary, exists := report.CheckSummaries[check.Name]
			if !exists {
				checkSummary = types.CheckSummary{
					Name:            check.Name,
					FailureMessages: make([]string, 0),
				}
			}

			checkSummary.TotalRuns++
			if check.Success {
				checkSummary.SuccessfulRuns++
			} else {
				checkSummary.FailedRuns++
				// Add failure message if not already present
				found := false
				for _, msg := range checkSummary.FailureMessages {
					if msg == check.FailureMessage {
						found = true
						break
					}
				}
				if !found && check.FailureMessage != "" {
					checkSummary.FailureMessages = append(checkSummary.FailureMessages, check.FailureMessage)
				}
			}

			report.CheckSummaries[check.Name] = checkSummary
		}

		// Add this request to the details
		report.RequestDetails = append(report.RequestDetails, result)
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

	// Derive start/end time and runtime if possible
	if !firstTimestamp.IsZero() {
		// Prefer provided startTime if non-zero, otherwise infer from first record
		if !startTime.IsZero() {
			report.StartTime = startTime
		} else {
			report.StartTime = firstTimestamp
		}
		// End at last observed timestamp
		report.EndTime = lastTimestamp
		if report.EndTime.After(report.StartTime) {
			report.RuntimeSeconds = report.EndTime.Sub(report.StartTime).Seconds()
		}
	}

	// Derive total virtual users from observed results
	report.TotalVirtualUsers = len(vus)

	// Populate key metrics summaries for offline reports
	if report.MetricsSummaries == nil {
		report.MetricsSummaries = make(map[string]metrics.MetricSummary)
	}

	// http_reqs counter with rate
	if report.TotalRequests > 0 {
		httpReqs := metrics.MetricSummary{
			Name:  "http_reqs",
			Type:  metrics.Counter,
			Count: report.TotalRequests,
			Sum:   float64(report.TotalRequests),
		}
		if report.RuntimeSeconds > 0 {
			httpReqs.Rate = httpReqs.Sum / report.RuntimeSeconds
			httpReqs.RateUnit = "req/s"
		}
		report.MetricsSummaries["http_reqs"] = httpReqs
	}

	// http_req_failed rate (average failure ratio)
	if report.TotalRequests > 0 {
		httpReqFailed := metrics.MetricSummary{
			Name:    "http_req_failed",
			Type:    metrics.Rate,
			Count:   report.TotalRequests,
			Sum:     float64(report.FailedRequests),
			Average: float64(report.FailedRequests) / float64(report.TotalRequests),
		}
		report.MetricsSummaries["http_req_failed"] = httpReqFailed
	}

	// http_req_duration trend (ms)
	if len(durationsMs) > 0 {
		// Compute stats
		sort.Float64s(durationsMs)
		var sum float64
		for _, v := range durationsMs {
			sum += v
		}
		count := len(durationsMs)
		durationSummary := metrics.MetricSummary{
			Name:    "http_req_duration",
			Type:    metrics.Trend,
			Count:   count,
			Sum:     sum,
			Average: sum / float64(count),
			Min:     durationsMs[0],
			Max:     durationsMs[count-1],
			P50:     percentile(durationsMs, 50),
			P90:     percentile(durationsMs, 90),
			P95:     percentile(durationsMs, 95),
			P99:     percentile(durationsMs, 99),
		}
		report.MetricsSummaries["http_req_duration"] = durationSummary
	}

	// checks rate (average success ratio)
	if report.TotalChecks > 0 {
		checks := metrics.MetricSummary{
			Name:    "checks",
			Type:    metrics.Rate,
			Count:   report.TotalChecks,
			Sum:     float64(report.SuccessfulChecks),
			Average: float64(report.SuccessfulChecks) / float64(report.TotalChecks),
		}
		report.MetricsSummaries["checks"] = checks
	}

	// Compute Top 5 Longest Requests by ResponseTime
	if len(report.RequestDetails) > 0 {
		// Copy slice to avoid mutating original order
		copied := make([]types.RequestResult, len(report.RequestDetails))
		copy(copied, report.RequestDetails)
		sort.Slice(copied, func(i, j int) bool {
			return copied[i].ResponseTime > copied[j].ResponseTime
		})
		limit := 5
		if len(copied) < limit {
			limit = len(copied)
		}
		report.TopLongestRequests = copied[:limit]
	}

	return report, nil
}

// GenerateHierarchicalReport creates a hierarchical report by streaming from the results file
func (fr *FileReporter) GenerateHierarchicalReport(startTime time.Time) (*types.HierarchicalReport, error) {
	return fr.generateHierarchicalReportStreaming(startTime)
}

// generateHierarchicalReportStreaming processes results in a single pass to optimize performance
func (fr *FileReporter) generateHierarchicalReportStreaming(startTime time.Time) (*types.HierarchicalReport, error) {
	// Single pass through the file to collect all necessary data
	summaryData, goroutineData, err := fr.collectAllDataInSinglePass()
	if err != nil {
		return nil, err
	}

	// Build summary report from collected data
	summaryReport := fr.buildSummaryFromData(summaryData, startTime)

	// Enrich summary with runtime and key metrics derived from request details
	fr.enrichSummaryWithOfflineMetrics(&summaryReport)

	// Build hierarchical report from goroutine data
	hierarchical := &types.HierarchicalReport{
		Summary:            summaryReport,
		VirtualUserReports: make([]types.GoroutineReport, 0, len(goroutineData)),
		TotalVirtualUsers:  len(goroutineData),
	}

	for goroutineID, iterations := range goroutineData {
		goroutineReport := fr.buildGoroutineReportFromDataWithResults(goroutineID, iterations, summaryData.requestDetails)
		hierarchical.VirtualUserReports = append(hierarchical.VirtualUserReports, goroutineReport)

		// Count successful goroutines
		if goroutineReport.SuccessfulIterations > 0 {
			hierarchical.SuccessfulVirtualUsers++
		} else {
			hierarchical.FailedVirtualUsers++
		}
	}

	return hierarchical, nil
}

// summaryData aggregates data for the summary report
type summaryData struct {
	totalRequests            int
	successfulRequests       int
	failedRequests           int
	totalResponseTime        time.Duration
	minResponseTime          time.Duration
	maxResponseTime          time.Duration
	responseTimeDistribution map[string]int
	errorBreakdown           map[string]int
	checkSummaries           map[string]types.CheckSummary
	totalChecks              int
	successfulChecks         int
	failedChecks             int
	requestDetails           []types.RequestResult
}

// iterationData holds aggregated data for an iteration
type iterationData struct {
	iterationID        int
	totalRequests      int
	successfulRequests int
	failedRequests     int
	totalResponseTime  time.Duration
	startTime          time.Time
	endTime            time.Time
}

// goroutineData holds aggregated data for a goroutine
type goroutineData struct {
	iterations map[int]*iterationData
}

// collectAllDataInSinglePass reads the file once and collects all necessary data
func (fr *FileReporter) collectAllDataInSinglePass() (*summaryData, map[int]*goroutineData, error) {
	file, err := os.Open(fr.resultsFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open results file: %v", err)
	}
	defer func() {
		_ = file.Close() // Ignore close error for read-only operations
	}()

	// Initialize summary data
	summary := &summaryData{
		minResponseTime:          time.Hour, // Initialize to high value
		responseTimeDistribution: make(map[string]int),
		errorBreakdown:           make(map[string]int),
		checkSummaries:           make(map[string]types.CheckSummary),
		requestDetails:           make([]types.RequestResult, 0),
	}

	// Initialize goroutine data map
	goroutines := make(map[int]*goroutineData)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var result types.RequestResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal result: %v", err)
		}

		// Update summary data
		fr.updateSummaryData(summary, result)

		// Update goroutine data
		fr.updateGoroutineData(goroutines, result)
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading file: %v", err)
	}

	// Reset min response time if no results
	if summary.totalRequests == 0 {
		summary.minResponseTime = 0
	}

	return summary, goroutines, nil
}

// updateSummaryData updates the summary data with a single result
func (fr *FileReporter) updateSummaryData(summary *summaryData, result types.RequestResult) {
	summary.totalRequests++
	summary.totalResponseTime += result.ResponseTime

	if result.Success {
		summary.successfulRequests++
	} else {
		summary.failedRequests++
		if result.Error != "" {
			summary.errorBreakdown[result.Error]++
		}
	}

	// Update min/max response times
	if result.ResponseTime < summary.minResponseTime {
		summary.minResponseTime = result.ResponseTime
	}
	if result.ResponseTime > summary.maxResponseTime {
		summary.maxResponseTime = result.ResponseTime
	}

	// Categorize response times
	ms := result.ResponseTime.Milliseconds()
	if ms < 100 {
		summary.responseTimeDistribution["<100ms"]++
	} else if ms < 500 {
		summary.responseTimeDistribution["100-500ms"]++
	} else if ms < 1000 {
		summary.responseTimeDistribution["500ms-1s"]++
	} else {
		summary.responseTimeDistribution[">1s"]++
	}

	// Process checks
	for _, check := range result.Checks {
		summary.totalChecks++
		if check.Success {
			summary.successfulChecks++
		} else {
			summary.failedChecks++
		}

		// Update check summary
		checkSummary, exists := summary.checkSummaries[check.Name]
		if !exists {
			checkSummary = types.CheckSummary{
				Name:            check.Name,
				FailureMessages: make([]string, 0),
			}
		}

		checkSummary.TotalRuns++
		if check.Success {
			checkSummary.SuccessfulRuns++
		} else {
			checkSummary.FailedRuns++
			if check.FailureMessage != "" && !containsString(checkSummary.FailureMessages, check.FailureMessage) {
				checkSummary.FailureMessages = append(checkSummary.FailureMessages, check.FailureMessage)
			}
		}

		summary.checkSummaries[check.Name] = checkSummary
	}

	// Add this request to the details
	summary.requestDetails = append(summary.requestDetails, result)
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// updateGoroutineData updates the goroutine data with a single result
func (fr *FileReporter) updateGoroutineData(goroutines map[int]*goroutineData, result types.RequestResult) {
	// Initialize goroutine if needed
	if goroutines[result.VirtualUserID] == nil {
		goroutines[result.VirtualUserID] = &goroutineData{
			iterations: make(map[int]*iterationData),
		}
	}

	// Initialize iteration if needed
	iteration := goroutines[result.VirtualUserID].iterations[result.IterationID]
	if iteration == nil {
		iteration = &iterationData{
			iterationID: result.IterationID,
			startTime:   result.Timestamp,
			endTime:     result.Timestamp,
		}
		goroutines[result.VirtualUserID].iterations[result.IterationID] = iteration
	}

	// Update iteration data
	iteration.totalRequests++
	iteration.totalResponseTime += result.ResponseTime

	if result.Success {
		iteration.successfulRequests++
	} else {
		iteration.failedRequests++
	}

	// Update time bounds
	if result.Timestamp.Before(iteration.startTime) {
		iteration.startTime = result.Timestamp
	}
	if result.Timestamp.After(iteration.endTime) {
		iteration.endTime = result.Timestamp
	}
}

// buildSummaryFromData builds a summary report from collected data
func (fr *FileReporter) buildSummaryFromData(summary *summaryData, startTime time.Time) types.Report {
	report := types.Report{
		TotalRequests:            summary.totalRequests,
		SuccessfulRequests:       summary.successfulRequests,
		FailedRequests:           summary.failedRequests,
		MinResponseTime:          summary.minResponseTime,
		MaxResponseTime:          summary.maxResponseTime,
		ResponseTimeDistribution: summary.responseTimeDistribution,
		ErrorBreakdown:           summary.errorBreakdown,
		CheckSummaries:           summary.checkSummaries,
		TotalChecks:              summary.totalChecks,
		SuccessfulChecks:         summary.successfulChecks,
		FailedChecks:             summary.failedChecks,
		RequestDetails:           summary.requestDetails,
		TopLongestRequests:       make([]types.RequestResult, 0),
		StartTime:                startTime,
		EndTime:                  time.Now(),
		MetricsSummaries:         make(map[string]metrics.MetricSummary),
	}

	// Calculate average response time
	if report.TotalRequests > 0 {
		report.AverageResponseTime = summary.totalResponseTime / time.Duration(report.TotalRequests)
	}

	return report
}

// enrichSummaryWithOfflineMetrics computes Start/End/Runtime and key metric summaries
// from the report's RequestDetails, for use when generating reports from raw results.
func (fr *FileReporter) enrichSummaryWithOfflineMetrics(report *types.Report) {
	if len(report.RequestDetails) == 0 {
		return
	}
	// Infer start/end from timestamps if available
	var firstTS time.Time
	var lastTS time.Time
	durations := make([]float64, 0, len(report.RequestDetails))
	for _, r := range report.RequestDetails {
		if firstTS.IsZero() || r.Timestamp.Before(firstTS) {
			firstTS = r.Timestamp
		}
		if lastTS.IsZero() || r.Timestamp.After(lastTS) {
			lastTS = r.Timestamp
		}
		durations = append(durations, float64(r.ResponseTime.Milliseconds()))
	}
	if report.StartTime.IsZero() {
		report.StartTime = firstTS
	}
	if !lastTS.IsZero() {
		report.EndTime = lastTS
	}
	if report.EndTime.After(report.StartTime) {
		report.RuntimeSeconds = report.EndTime.Sub(report.StartTime).Seconds()
	}

	if report.MetricsSummaries == nil {
		report.MetricsSummaries = make(map[string]metrics.MetricSummary)
	}

	// http_reqs
	if report.TotalRequests > 0 {
		httpReqs := metrics.MetricSummary{
			Name:  "http_reqs",
			Type:  metrics.Counter,
			Count: report.TotalRequests,
			Sum:   float64(report.TotalRequests),
		}
		if report.RuntimeSeconds > 0 {
			httpReqs.Rate = httpReqs.Sum / report.RuntimeSeconds
			httpReqs.RateUnit = "req/s"
		}
		report.MetricsSummaries["http_reqs"] = httpReqs
	}

	// http_req_failed
	if report.TotalRequests > 0 {
		httpReqFailed := metrics.MetricSummary{
			Name:    "http_req_failed",
			Type:    metrics.Rate,
			Count:   report.TotalRequests,
			Sum:     float64(report.FailedRequests),
			Average: float64(report.FailedRequests) / float64(report.TotalRequests),
		}
		report.MetricsSummaries["http_req_failed"] = httpReqFailed
	}

	// http_req_duration
	if len(durations) > 0 {
		sort.Float64s(durations)
		var sum float64
		for _, v := range durations {
			sum += v
		}
		count := len(durations)
		report.MetricsSummaries["http_req_duration"] = metrics.MetricSummary{
			Name:    "http_req_duration",
			Type:    metrics.Trend,
			Count:   count,
			Sum:     sum,
			Average: sum / float64(count),
			Min:     durations[0],
			Max:     durations[count-1],
			P50:     percentile(durations, 50),
			P90:     percentile(durations, 90),
			P95:     percentile(durations, 95),
			P99:     percentile(durations, 99),
		}
	}

	// checks
	if report.TotalChecks > 0 {
		report.MetricsSummaries["checks"] = metrics.MetricSummary{
			Name:    "checks",
			Type:    metrics.Rate,
			Count:   report.TotalChecks,
			Sum:     float64(report.SuccessfulChecks),
			Average: float64(report.SuccessfulChecks) / float64(report.TotalChecks),
		}
	}

	// Compute Top 5 Longest Requests by ResponseTime
	if len(report.RequestDetails) > 0 {
		copied := make([]types.RequestResult, len(report.RequestDetails))
		copy(copied, report.RequestDetails)
		sort.Slice(copied, func(i, j int) bool {
			return copied[i].ResponseTime > copied[j].ResponseTime
		})
		limit := 5
		if len(copied) < limit {
			limit = len(copied)
		}
		report.TopLongestRequests = copied[:limit]
	}
}

// percentile calculates an approximate percentile value from a sorted slice
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	// Clamp p
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	idx := int(p / 100.0 * float64(len(sorted)-1))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// buildGoroutineReportFromDataWithResults builds a goroutine report from collected data including request results
func (fr *FileReporter) buildGoroutineReportFromDataWithResults(goroutineID int, goroutineData *goroutineData, allRequestResults []types.RequestResult) types.GoroutineReport {
	report := types.GoroutineReport{
		GoroutineID:     goroutineID,
		Iterations:      make([]types.IterationReport, 0, len(goroutineData.iterations)),
		TotalIterations: len(goroutineData.iterations),
		StartTime:       time.Now(),
		EndTime:         time.Time{},
	}

	var totalResponseTime time.Duration

	// Sort iterations by ID to ensure consistent order
	iterationIDs := make([]int, 0, len(goroutineData.iterations))
	for id := range goroutineData.iterations {
		iterationIDs = append(iterationIDs, id)
	}
	sort.Ints(iterationIDs)

	// Process each iteration
	for _, iterationID := range iterationIDs {
		iteration := goroutineData.iterations[iterationID]
		// Find request results for this specific goroutine and iteration
		var iterationRequestResults []types.RequestResult
		for _, result := range allRequestResults {
			if result.VirtualUserID == goroutineID && result.IterationID == iteration.iterationID {
				iterationRequestResults = append(iterationRequestResults, result)
			}
		}

		iterationReport := types.IterationReport{
			IterationID:        iteration.iterationID,
			RequestResults:     iterationRequestResults, // Now populated with actual results
			TotalRequests:      iteration.totalRequests,
			SuccessfulRequests: iteration.successfulRequests,
			FailedRequests:     iteration.failedRequests,
			StartTime:          iteration.startTime,
			EndTime:            iteration.endTime,
			TotalDuration:      iteration.endTime.Sub(iteration.startTime),
		}

		// Calculate average response time for iteration
		if iteration.totalRequests > 0 {
			iterationReport.AverageResponseTime = iteration.totalResponseTime / time.Duration(iteration.totalRequests)
		}

		report.Iterations = append(report.Iterations, iterationReport)

		// Aggregate goroutine stats
		report.TotalRequests += iteration.totalRequests
		report.SuccessfulRequests += iteration.successfulRequests
		report.FailedRequests += iteration.failedRequests
		totalResponseTime += iteration.totalResponseTime

		// Count successful iterations
		if iteration.failedRequests == 0 && iteration.totalRequests > 0 {
			report.SuccessfulIterations++
		} else {
			report.FailedIterations++
		}

		// Track time bounds
		if iteration.startTime.Before(report.StartTime) || report.StartTime.IsZero() {
			report.StartTime = iteration.startTime
		}
		if iteration.endTime.After(report.EndTime) {
			report.EndTime = iteration.endTime
		}
	}

	// Calculate averages
	if report.TotalRequests > 0 {
		report.AverageResponseTime = totalResponseTime / time.Duration(report.TotalRequests)
	}
	report.TotalDuration = report.EndTime.Sub(report.StartTime)

	return report
}
