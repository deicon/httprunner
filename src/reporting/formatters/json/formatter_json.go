package json

import (
	"encoding/json"
	"time"

	"github.com/deicon/httprunner/src/reporting/types"
)

// JSONFormatter formats reports as JSON
type JSONFormatter struct{}

func (f *JSONFormatter) Format(report *types.Report) (string, error) {
	// Create a more structured JSON output
	output := map[string]interface{}{
		"summary": map[string]interface{}{
			"totalRequests":       report.TotalRequests,
			"successfulRequests":  report.SuccessfulRequests,
			"failedRequests":      report.FailedRequests,
			"successRate":         0.0,
			"totalChecks":         report.TotalChecks,
			"successfulChecks":    report.SuccessfulChecks,
			"failedChecks":        report.FailedChecks,
			"checkSuccessRate":    0.0,
			"totalVirtualUsers":   report.TotalVirtualUsers,
			"runtimeSeconds":      report.RuntimeSeconds,
			"requestsPerSecond":   0.0,
			"averageResponseTime": report.AverageResponseTime.String(),
			"minResponseTime":     report.MinResponseTime.String(),
			"maxResponseTime":     report.MaxResponseTime.String(),
			"totalDuration":       report.EndTime.Sub(report.StartTime).String(),
			"startTime":           report.StartTime.Format(time.RFC3339),
			"endTime":             report.EndTime.Format(time.RFC3339),
			"topLongestRequests":  report.TopLongestRequests,
		},
		"responseTimeDistribution": report.ResponseTimeDistribution,
		"errorBreakdown":           report.ErrorBreakdown,
		"checkSummaries":           report.CheckSummaries,
		"metricsSummaries":         report.MetricsSummaries,
		"perVUMetrics":             report.PerVUMetrics,
		"perIterationMetrics":      report.PerIterationMetrics,
		"requestDetails":           report.RequestDetails,
	}

	if report.TotalRequests > 0 {
		output["summary"].(map[string]interface{})["successRate"] = float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
	}

	if report.RuntimeSeconds > 0 {
		output["summary"].(map[string]interface{})["requestsPerSecond"] = float64(report.TotalRequests) / report.RuntimeSeconds
	}

	if report.TotalChecks > 0 {
		output["summary"].(map[string]interface{})["checkSuccessRate"] = float64(report.SuccessfulChecks) / float64(report.TotalChecks) * 100
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}
