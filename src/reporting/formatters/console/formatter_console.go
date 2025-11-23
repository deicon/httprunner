package console

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/deicon/httprunner/src/metrics"
	"github.com/deicon/httprunner/src/reporting/types"
)

// ConsoleFormatter formats reports for console output
type ConsoleFormatter struct{}

func (f *ConsoleFormatter) Format(report *types.Report) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("HTTP Request Report\n")
	buf.WriteString("===================\n\n")

	// Summary statistics
	buf.WriteString(fmt.Sprintf("Total Requests: %d", report.TotalRequests))
	if report.RuntimeSeconds > 0 {
		reqPerSec := float64(report.TotalRequests) / report.RuntimeSeconds
		buf.WriteString(fmt.Sprintf(" (%.1f req/s)", reqPerSec))
	}
	buf.WriteString("\n")

	buf.WriteString(fmt.Sprintf("Successful Requests: %d\n", report.SuccessfulRequests))
	buf.WriteString(fmt.Sprintf("Failed Requests: %d\n", report.FailedRequests))

	if report.TotalRequests > 0 {
		successRate := float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
		buf.WriteString(fmt.Sprintf("Success Rate: %.1f%%\n", successRate))
	}

	buf.WriteString(fmt.Sprintf("Virtual Users: %d\n", report.TotalVirtualUsers))
	buf.WriteString(fmt.Sprintf("Runtime: %.1fs\n", report.RuntimeSeconds))
	buf.WriteString(fmt.Sprintf("Average Response Time: %v\n", report.AverageResponseTime))
	buf.WriteString(fmt.Sprintf("Minimum Response Time: %v\n", report.MinResponseTime))
	buf.WriteString(fmt.Sprintf("Maximum Response Time: %v\n", report.MaxResponseTime))
	buf.WriteString(fmt.Sprintf("Total Duration: %v\n\n", report.EndTime.Sub(report.StartTime)))

	// Top longest requests
	if len(report.TopLongestRequests) > 0 {
		buf.WriteString("Top 5 Longest Requests:\n")
		limit := 5
		if len(report.TopLongestRequests) < limit {
			limit = len(report.TopLongestRequests)
		}
		for i := 0; i < limit; i++ {
			req := report.TopLongestRequests[i]
			status := fmt.Sprintf("%d ✓", req.StatusCode)
			if !req.Success {
				status = fmt.Sprintf("ERROR: %s", req.Error)
			}
			buf.WriteString(fmt.Sprintf("  %d. %s %s %s  %s  %.3f ms  (VU %d, Iter %d)\n",
				i+1, req.Verb, req.Name, req.URL, status, float64(req.ResponseTime.Nanoseconds())/1e6, req.VirtualUserID, req.IterationID))
		}
		buf.WriteString("\n")
	}

	// Response time distribution
	if len(report.ResponseTimeDistribution) > 0 {
		buf.WriteString("Response Time Distribution:\n")
		// Sort categories for consistent output
		categories := []string{"<100ms", "100-500ms", "500ms-1s", ">1s"}
		for _, category := range categories {
			if count, exists := report.ResponseTimeDistribution[category]; exists && count > 0 {
				buf.WriteString(fmt.Sprintf("- %s: %d\n", category, count))
			}
		}
		buf.WriteString("\n")
	}

	// Error breakdown
	if len(report.ErrorBreakdown) > 0 {
		buf.WriteString("Error Breakdown:\n")
		for errorType, count := range report.ErrorBreakdown {
			buf.WriteString(fmt.Sprintf("- %s: %d\n", errorType, count))
		}
		buf.WriteString("\n")
	}

	// Check results summary
	if len(report.CheckSummaries) > 0 {
		buf.WriteString("Check Results:\n")
		buf.WriteString(fmt.Sprintf("Total Checks: %d\n", report.TotalChecks))
		buf.WriteString(fmt.Sprintf("Successful Checks: %d\n", report.SuccessfulChecks))
		buf.WriteString(fmt.Sprintf("Failed Checks: %d\n", report.FailedChecks))

		if report.TotalChecks > 0 {
			checkSuccessRate := float64(report.SuccessfulChecks) / float64(report.TotalChecks) * 100
			buf.WriteString(fmt.Sprintf("Check Success Rate: %.1f%%\n", checkSuccessRate))
		}
		buf.WriteString("\n")

		buf.WriteString("Check Breakdown:\n")
		for checkName, summary := range report.CheckSummaries {
			buf.WriteString(fmt.Sprintf("- %s: %d runs (%d successful, %d failed)\n",
				checkName, summary.TotalRuns, summary.SuccessfulRuns, summary.FailedRuns))
			if len(summary.FailureMessages) > 0 {
				buf.WriteString(fmt.Sprintf("  Failure reasons: %v\n", summary.FailureMessages))
			}
		}
		buf.WriteString("\n")
	}

	// All collected metrics summary
	if len(report.MetricsSummaries) > 0 {
		buf.WriteString("All Collected Metrics:\n")
		buf.WriteString("=====================\n\n")

		// Group metrics by type for better organization
		counters := make(map[string]metrics.MetricSummary)
		trends := make(map[string]metrics.MetricSummary)
		rates := make(map[string]metrics.MetricSummary)
		gauges := make(map[string]metrics.MetricSummary)

		for name, summary := range report.MetricsSummaries {
			switch summary.Type {
			case "counter":
				counters[name] = summary
			case "trend":
				trends[name] = summary
			case "rate":
				rates[name] = summary
			case "gauge":
				gauges[name] = summary
			}
		}

		// Display counters with rates
		if len(counters) > 0 {
			buf.WriteString("Counters:\n")
			for name, summary := range counters {
				rateInfo := ""
				if summary.Rate > 0 && summary.RateUnit != "" {
					rateInfo = fmt.Sprintf("  (%.1f %s)", summary.Rate, summary.RateUnit)
				}
				buf.WriteString(fmt.Sprintf("  %-25s %10.0f  (count: %d)%s\n", name, summary.Sum, summary.Count, rateInfo))
			}
			buf.WriteString("\n")
		}

		// Display trends (histograms)
		if len(trends) > 0 {
			buf.WriteString("Trends:\n")
			for name, summary := range trends {
				unit := "ms"
				if name == "data_sent" || name == "data_received" {
					unit = "bytes"
				}
				buf.WriteString(fmt.Sprintf("  %-25s avg=%.2f%s min=%.2f%s med=%.2f%s max=%.2f%s p(90)=%.2f%s p(95)=%.2f%s\n",
					name, summary.Average, unit, summary.Min, unit, summary.P50, unit, summary.Max, unit, summary.P90, unit, summary.P95, unit))
			}
			buf.WriteString("\n")
		}

		// Display rates
		if len(rates) > 0 {
			buf.WriteString("Rates:\n")
			for name, summary := range rates {
				rate := summary.Average * 100 // Convert to percentage
				// For http_req_failed, show number of failed requests (sum of 1/0 values)
				if name == "http_req_failed" {
					buf.WriteString(fmt.Sprintf("  %-25s %.2f%%  (count: %d)\n", name, rate, int(summary.Sum)))
					continue
				}
				buf.WriteString(fmt.Sprintf("  %-25s %.2f%%  (count: %d)\n", name, rate, summary.Count))
			}
			buf.WriteString("\n")
		}

		// Display gauges
		if len(gauges) > 0 {
			buf.WriteString("Gauges:\n")
			for name, summary := range gauges {
				buf.WriteString(fmt.Sprintf("  %-25s %.0f  (latest: %.0f)\n", name, summary.Average, summary.LatestValue))
			}
			buf.WriteString("\n")
		}
	}

	// Per-VU metrics
	if len(report.PerVUMetrics) > 0 {
		buf.WriteString("Per Virtual User Metrics:\n")
		buf.WriteString("========================\n")
		for name, value := range report.PerVUMetrics {
			displayName := strings.Replace(name, "_per_vu", "", 1)
			buf.WriteString(fmt.Sprintf("  %-25s %.2f\n", displayName, value))
		}
		buf.WriteString("\n")
	}

	// Per-iteration metrics
	if len(report.PerIterationMetrics) > 0 {
		buf.WriteString("Per Iteration Metrics:\n")
		buf.WriteString("=====================\n")
		for name, value := range report.PerIterationMetrics {
			displayName := strings.Replace(name, "_per_iter", "", 1)
			buf.WriteString(fmt.Sprintf("  %-25s %.2f\n", displayName, value))
		}
		buf.WriteString("\n")
	}

	// Request details (limited to first 10 for console)
	if len(report.RequestDetails) > 0 {
		buf.WriteString("Request Details (showing first 10):\n")
		limit := 10
		if len(report.RequestDetails) < limit {
			limit = len(report.RequestDetails)
		}

		for i := 0; i < limit; i++ {
			req := report.RequestDetails[i]
			status := fmt.Sprintf("%d ✓", req.StatusCode)
			if !req.Success {
				status = fmt.Sprintf("ERROR: %s", req.Error)
			}

			// Build check failure info for this request
			checkFailureInfo := ""
			failedChecks := make([]string, 0)
			for _, check := range req.Checks {
				if !check.Success {
					failedChecks = append(failedChecks, check.FailureMessage)
				}
			}
			if len(failedChecks) > 0 {
				checkFailureInfo = fmt.Sprintf("    %s Check Failed (Failures: %s)", req.Timestamp.Format("15:04:05.000"), strings.Join(failedChecks, ", "))
			}

			buf.WriteString(fmt.Sprintf("  %d    %s    %s    %s    %s    %.3f ms%s\n",
				i+1, req.Name, req.Verb, req.URL, status, float64(req.ResponseTime.Nanoseconds())/1000000.0, checkFailureInfo))
		}

		if len(report.RequestDetails) > limit {
			buf.WriteString(fmt.Sprintf("... and %d more requests\n", len(report.RequestDetails)-limit))
		}
	}

	return buf.String(), nil
}
