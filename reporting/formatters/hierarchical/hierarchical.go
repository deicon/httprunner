package hierarchical

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/deicon/httprunner/metrics"
	"github.com/deicon/httprunner/reporting/types"
)

// HierarchicalFormatter formats hierarchical reports based on detail level
type HierarchicalFormatter struct {
	DetailLevel types.ReportDetailLevel
	Format      types.ReportFormat
}

func (f *HierarchicalFormatter) FormatHierarchical(report *types.HierarchicalReport) (string, error) {
	switch f.Format {
	case types.FormatConsole:
		return f.formatHierarchicalConsole(report)
	case types.FormatHTML:
		return f.formatHierarchicalHTML(report)
	case types.FormatJSON:
		return f.formatHierarchicalJSON(report)
	default:
		// Fallback to regular console format
		return f.formatHierarchicalConsole(report)
	}
}

func (f *HierarchicalFormatter) formatHierarchicalConsole(report *types.HierarchicalReport) (string, error) {
	var buf bytes.Buffer

	// Always show summary
	buf.WriteString("HTTP Request Report - Summary\n")
	buf.WriteString("============================\n\n")

	buf.WriteString(fmt.Sprintf("Total VirtualUserReports: %d\n", report.TotalVirtualUsers))
	buf.WriteString(fmt.Sprintf("Successful VirtualUserReports: %d\n", report.SuccessfulVirtualUsers))
	buf.WriteString(fmt.Sprintf("Failed VirtualUserReports: %d\n", report.FailedVirtualUsers))
	if report.TotalVirtualUsers > 0 {
		successRate := float64(report.SuccessfulVirtualUsers) / float64(report.TotalVirtualUsers) * 100
		buf.WriteString(fmt.Sprintf("Goroutine Success Rate: %.1f%%\n\n", successRate))
	}

	buf.WriteString(fmt.Sprintf("Total Requests: %d\n", report.Summary.TotalRequests))
	buf.WriteString(fmt.Sprintf("Successful Requests: %d\n", report.Summary.SuccessfulRequests))
	buf.WriteString(fmt.Sprintf("Failed Requests: %d\n", report.Summary.FailedRequests))

	if report.Summary.TotalRequests > 0 {
		successRate := float64(report.Summary.SuccessfulRequests) / float64(report.Summary.TotalRequests) * 100
		buf.WriteString(fmt.Sprintf("Request Success Rate: %.1f%%\n", successRate))
	}

	buf.WriteString(fmt.Sprintf("Average Response Time: %v\n", report.Summary.AverageResponseTime))
	buf.WriteString(fmt.Sprintf("Total Duration: %v\n\n", report.Summary.EndTime.Sub(report.Summary.StartTime)))

	// Check results summary
	if len(report.Summary.CheckSummaries) > 0 {
		buf.WriteString("Check Results:\n")
		buf.WriteString(fmt.Sprintf("Total Checks: %d\n", report.Summary.TotalChecks))
		buf.WriteString(fmt.Sprintf("Successful Checks: %d\n", report.Summary.SuccessfulChecks))
		buf.WriteString(fmt.Sprintf("Failed Checks: %d\n", report.Summary.FailedChecks))

		if report.Summary.TotalChecks > 0 {
			checkSuccessRate := float64(report.Summary.SuccessfulChecks) / float64(report.Summary.TotalChecks) * 100
			buf.WriteString(fmt.Sprintf("Check Success Rate: %.1f%%\n", checkSuccessRate))
		}
		buf.WriteString("\n")

		buf.WriteString("Check Breakdown:\n")
		for checkName, summary := range report.Summary.CheckSummaries {
			buf.WriteString(fmt.Sprintf("- %s: %d runs (%d successful, %d failed)\n",
				checkName, summary.TotalRuns, summary.SuccessfulRuns, summary.FailedRuns))
			if len(summary.FailureMessages) > 0 {
				buf.WriteString(fmt.Sprintf("  Failure reasons: %v\n", summary.FailureMessages))
			}
		}
		buf.WriteString("\n")
	}

	// All collected metrics summary
	if len(report.Summary.MetricsSummaries) > 0 {
		buf.WriteString("All Collected Metrics:\n")
		buf.WriteString("=====================\n\n")

		// Group metrics by type for better organization
		counters := make(map[string]metrics.MetricSummary)
		trends := make(map[string]metrics.MetricSummary)
		rates := make(map[string]metrics.MetricSummary)
		gauges := make(map[string]metrics.MetricSummary)

		for name, summary := range report.Summary.MetricsSummaries {
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

		// Display counters
		if len(counters) > 0 {
			buf.WriteString("Counters:\n")
			for name, summary := range counters {
				buf.WriteString(fmt.Sprintf("  %-25s %10.0f  (count: %d)\n", name, summary.Sum, summary.Count))
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

	// Show virtual users details if requested
	if f.DetailLevel == types.DetailVirtualuser || f.DetailLevel == types.DetailIteration || f.DetailLevel == types.DetailFull {
		buf.WriteString("Goroutine Breakdown\n")
		buf.WriteString("===================\n\n")

		for _, gr := range report.VirtualUserReports {
			buf.WriteString(fmt.Sprintf("Goroutine %d:\n", gr.GoroutineID))
			buf.WriteString(fmt.Sprintf("  Total Iterations: %d\n", gr.TotalIterations))
			buf.WriteString(fmt.Sprintf("  Successful Iterations: %d\n", gr.SuccessfulIterations))
			buf.WriteString(fmt.Sprintf("  Failed Iterations: %d\n", gr.FailedIterations))
			buf.WriteString(fmt.Sprintf("  Total Requests: %d\n", gr.TotalRequests))
			buf.WriteString(fmt.Sprintf("  Success Rate: %.1f%%\n", float64(gr.SuccessfulRequests)/float64(gr.TotalRequests)*100))
			buf.WriteString(fmt.Sprintf("  Average Response Time: %v\n", gr.AverageResponseTime))
			buf.WriteString(fmt.Sprintf("  Duration: %v\n\n", gr.TotalDuration))
		}
	}

	// Show iteration details if requested
	if f.DetailLevel == types.DetailIteration || f.DetailLevel == types.DetailFull {
		buf.WriteString("Iteration Breakdown\n")
		buf.WriteString("===================\n\n")

		for _, gr := range report.VirtualUserReports {
			if len(gr.Iterations) > 0 {
				buf.WriteString(fmt.Sprintf("Goroutine %d Iterations:\n", gr.GoroutineID))
				for _, iter := range gr.Iterations {
					// Collect failed checks for this iteration
					failedChecksList := make([]string, 0)
					for _, request := range iter.RequestResults {
						for _, check := range request.Checks {
							if !check.Success {
								failedChecksList = append(failedChecksList, check.FailureMessage)
							}
						}
					}

					buf.WriteString(fmt.Sprintf("  Iteration %d:\n", iter.IterationID))
					buf.WriteString(fmt.Sprintf("    Requests: %d (Success: %d, Failed: %d)\n",
						iter.TotalRequests, iter.SuccessfulRequests, iter.FailedRequests))
					buf.WriteString(fmt.Sprintf("    Average Response Time: %v\n", iter.AverageResponseTime))
					buf.WriteString(fmt.Sprintf("    Duration: %v\n", iter.TotalDuration))

					if len(failedChecksList) > 0 {
						buf.WriteString(fmt.Sprintf("    Check Failures: %s\n", strings.Join(failedChecksList, ", ")))
					}
				}
				buf.WriteString("\n")
			}
		}
	}

	// Show full request details if requested
	if f.DetailLevel == types.DetailFull {
		buf.WriteString("Request Details\n")
		buf.WriteString("===============\n\n")

		count := 1
		for _, gr := range report.VirtualUserReports {
			for _, iter := range gr.Iterations {
				if len(iter.RequestResults) > 0 {
					buf.WriteString(fmt.Sprintf("Goroutine %d, Iteration %d:\n", gr.GoroutineID, iter.IterationID))
					for _, req := range iter.RequestResults {
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
							count, req.Name, req.Verb, req.URL, status, float64(req.ResponseTime.Nanoseconds())/1000000.0, checkFailureInfo))
						count++
					}
					buf.WriteString("\n")
				}
			}
		}
	}

	return buf.String(), nil
}

func (f *HierarchicalFormatter) formatHierarchicalJSON(report *types.HierarchicalReport) (string, error) {
	// Create structured JSON based on detail level
	output := map[string]interface{}{
		"summary": map[string]interface{}{
			"totalGoroutines":      report.TotalVirtualUsers,
			"successfulGoroutines": report.SuccessfulVirtualUsers,
			"failedGoroutines":     report.FailedVirtualUsers,
			"totalRequests":        report.Summary.TotalRequests,
			"successfulRequests":   report.Summary.SuccessfulRequests,
			"failedRequests":       report.Summary.FailedRequests,
			"averageResponseTime":  report.Summary.AverageResponseTime.String(),
			"totalDuration":        report.Summary.EndTime.Sub(report.Summary.StartTime).String(),
			"startTime":            report.Summary.StartTime.Format(time.RFC3339),
			"endTime":              report.Summary.EndTime.Format(time.RFC3339),
		},
	}

	if f.DetailLevel == types.DetailVirtualuser || f.DetailLevel == types.DetailIteration || f.DetailLevel == types.DetailFull {
		goroutines := make([]map[string]interface{}, 0, len(report.VirtualUserReports))
		for _, gr := range report.VirtualUserReports {
			goroutine := map[string]interface{}{
				"goroutineID":          gr.GoroutineID,
				"totalIterations":      gr.TotalIterations,
				"successfulIterations": gr.SuccessfulIterations,
				"failedIterations":     gr.FailedIterations,
				"totalRequests":        gr.TotalRequests,
				"successfulRequests":   gr.SuccessfulRequests,
				"failedRequests":       gr.FailedRequests,
				"averageResponseTime":  gr.AverageResponseTime.String(),
				"totalDuration":        gr.TotalDuration.String(),
			}

			if f.DetailLevel == types.DetailIteration || f.DetailLevel == types.DetailFull {
				iterations := make([]map[string]interface{}, 0, len(gr.Iterations))
				for _, iter := range gr.Iterations {
					iteration := map[string]interface{}{
						"iterationID":         iter.IterationID,
						"totalRequests":       iter.TotalRequests,
						"successfulRequests":  iter.SuccessfulRequests,
						"failedRequests":      iter.FailedRequests,
						"averageResponseTime": iter.AverageResponseTime.String(),
						"totalDuration":       iter.TotalDuration.String(),
					}

					if f.DetailLevel == types.DetailFull {
						iteration["requestResults"] = iter.RequestResults
					}

					iterations = append(iterations, iteration)
				}
				goroutine["iterations"] = iterations
			}

			goroutines = append(goroutines, goroutine)
		}
		output["goroutines"] = goroutines
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

//go:embed templates/hierarchicalHTMLTemplate.html
var hierarchicalHTMLTemplate string

func (f *HierarchicalFormatter) formatHierarchicalHTML(report *types.HierarchicalReport) (string, error) {

	tmpl, err := template.New("hierarchical").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b float64) float64 { return a * b },
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"float64": func(i int) float64 { return float64(i) },
		"collectFailedChecks": func(requests []types.RequestResult) string {
			var failedChecks []string
			for _, request := range requests {
				for _, check := range request.Checks {
					if !check.Success {
						failedChecks = append(failedChecks, check.FailureMessage)
					}
				}
			}
			if len(failedChecks) > 0 {
				return strings.Join(failedChecks, ", ")
			}
			return ""
		},
	}).Parse(hierarchicalHTMLTemplate)
	if err != nil {
		return "", err
	}

	// Prepare template data
	data := struct {
		*types.HierarchicalReport
		SuccessRate   float64
		TotalDuration time.Duration
		DetailLevel   string
	}{
		HierarchicalReport: report,
		TotalDuration:      report.Summary.EndTime.Sub(report.Summary.StartTime),
		DetailLevel:        string(f.DetailLevel),
	}

	if report.Summary.TotalRequests > 0 {
		data.SuccessRate = float64(report.Summary.SuccessfulRequests) / float64(report.Summary.TotalRequests) * 100
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
