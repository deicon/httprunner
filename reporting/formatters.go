package reporting

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/deicon/httprunner/metrics"
)

// Formatter interface for different report formats
type Formatter interface {
	Format(report *Report) (string, error)
}

// ConsoleFormatter formats reports for console output
type ConsoleFormatter struct{}

func (f *ConsoleFormatter) Format(report *Report) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("HTTP Request Report\n")
	buf.WriteString("===================\n\n")

	// Summary statistics
	buf.WriteString(fmt.Sprintf("Total Requests: %d\n", report.TotalRequests))
	buf.WriteString(fmt.Sprintf("Successful Requests: %d\n", report.SuccessfulRequests))
	buf.WriteString(fmt.Sprintf("Failed Requests: %d\n", report.FailedRequests))

	if report.TotalRequests > 0 {
		successRate := float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
		buf.WriteString(fmt.Sprintf("Success Rate: %.1f%%\n", successRate))
	}

	buf.WriteString(fmt.Sprintf("Average Response Time: %v\n", report.AverageResponseTime))
	buf.WriteString(fmt.Sprintf("Minimum Response Time: %v\n", report.MinResponseTime))
	buf.WriteString(fmt.Sprintf("Maximum Response Time: %v\n", report.MaxResponseTime))
	buf.WriteString(fmt.Sprintf("Total Duration: %v\n\n", report.EndTime.Sub(report.StartTime)))

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

// HTMLFormatter formats reports as HTML
type HTMLFormatter struct{}

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>HTTP Request Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .summary { background: #f5f5f5; padding: 15px; border-radius: 5px; margin-bottom: 20px; }
        .metric { display: inline-block; margin-right: 30px; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .success { color: green; }
        .error { color: red; }
        .distribution, .errors { display: inline-block; vertical-align: top; margin-right: 30px; }
        .chart { margin: 20px 0; }
    </style>
</head>
<body>
    <h1>HTTP Request Report</h1>
    
    <div class="summary">
        <div class="metric"><strong>Total Requests:</strong> {{.TotalRequests}}</div>
        <div class="metric"><strong>Successful:</strong> <span class="success">{{.SuccessfulRequests}}</span></div>
        <div class="metric"><strong>Failed:</strong> <span class="error">{{.FailedRequests}}</span></div>
        <div class="metric"><strong>Success Rate:</strong> {{printf "%.1f" .SuccessRate}}%</div>
    </div>

    <div class="summary">
        <div class="metric"><strong>Avg Response Time:</strong> {{.AverageResponseTime}}</div>
        <div class="metric"><strong>Min Response Time:</strong> {{.MinResponseTime}}</div>
        <div class="metric"><strong>Max Response Time:</strong> {{.MaxResponseTime}}</div>
        <div class="metric"><strong>Total Duration:</strong> {{.TotalDuration}}</div>
    </div>

    {{if .ResponseTimeDistribution}}
    <div class="chart">
        <div class="distribution">
            <h3>Response Time Distribution</h3>
            <ul>
                {{range $category, $count := .ResponseTimeDistribution}}
                {{if gt $count 0}}
                <li>{{$category}}: {{$count}}</li>
                {{end}}
                {{end}}
            </ul>
        </div>
    </div>
    {{end}}

    {{if .ErrorBreakdown}}
    <div class="chart">
        <div class="errors">
            <h3>Error Breakdown</h3>
            <ul>
                {{range $error, $count := .ErrorBreakdown}}
                <li>{{$error}}: {{$count}}</li>
                {{end}}
            </ul>
        </div>
    </div>
    {{end}}

    {{if .CheckSummaries}}
    <div class="summary">
        <div class="metric"><strong>Total Checks:</strong> {{.TotalChecks}}</div>
        <div class="metric"><strong>Successful Checks:</strong> <span class="success">{{.SuccessfulChecks}}</span></div>
        <div class="metric"><strong>Failed Checks:</strong> <span class="error">{{.FailedChecks}}</span></div>
        <div class="metric"><strong>Check Success Rate:</strong> {{if gt .TotalChecks 0}}{{printf "%.1f" .CheckSuccessRate}}%{{else}}N/A{{end}}</div>
    </div>

    <div class="chart">
        <div class="errors">
            <h3>Check Results</h3>
            <ul>
                {{range $name, $summary := .CheckSummaries}}
                <li>
                    <strong>{{$name}}</strong>: {{$summary.TotalRuns}} runs 
                    ({{$summary.SuccessfulRuns}} successful, {{$summary.FailedRuns}} failed)
                    {{if gt (len $summary.FailureMessages) 0}}
                    <br><small>Failure reasons: {{range $i, $msg := $summary.FailureMessages}}{{if $i}}, {{end}}{{$msg}}{{end}}</small>
                    {{end}}
                </li>
                {{end}}
            </ul>
        </div>
    </div>
    {{end}}

    {{if .MetricsSummaries}}
    <div class="summary">
        <h3>📊 All Collected Metrics</h3>
        
        <h4>Counters</h4>
        <table style="margin-bottom: 20px;">
            <tr><th>Metric</th><th>Value</th><th>Count</th></tr>
            {{range $name, $summary := .MetricsSummaries}}
                {{if eq $summary.Type "counter"}}
                <tr><td>{{$name}}</td><td>{{printf "%.0f" $summary.Sum}}</td><td>{{$summary.Count}}</td></tr>
                {{end}}
            {{end}}
        </table>
        
        <h4>Trends (Timing/Size Metrics)</h4>
        <table style="margin-bottom: 20px;">
            <tr><th>Metric</th><th>Avg</th><th>Min</th><th>Med</th><th>Max</th><th>P90</th><th>P95</th></tr>
            {{range $name, $summary := .MetricsSummaries}}
                {{if eq $summary.Type "trend"}}
                <tr>
                    <td>{{$name}}</td>
                    <td>{{if or (eq $name "data_sent") (eq $name "data_received")}}{{printf "%.0f bytes" $summary.Average}}{{else}}{{printf "%.2f ms" $summary.Average}}{{end}}</td>
                    <td>{{if or (eq $name "data_sent") (eq $name "data_received")}}{{printf "%.0f" $summary.Min}}{{else}}{{printf "%.2f" $summary.Min}}{{end}}</td>
                    <td>{{if or (eq $name "data_sent") (eq $name "data_received")}}{{printf "%.0f" $summary.P50}}{{else}}{{printf "%.2f" $summary.P50}}{{end}}</td>
                    <td>{{if or (eq $name "data_sent") (eq $name "data_received")}}{{printf "%.0f" $summary.Max}}{{else}}{{printf "%.2f" $summary.Max}}{{end}}</td>
                    <td>{{if or (eq $name "data_sent") (eq $name "data_received")}}{{printf "%.0f" $summary.P90}}{{else}}{{printf "%.2f" $summary.P90}}{{end}}</td>
                    <td>{{if or (eq $name "data_sent") (eq $name "data_received")}}{{printf "%.0f" $summary.P95}}{{else}}{{printf "%.2f" $summary.P95}}{{end}}</td>
                </tr>
                {{end}}
            {{end}}
        </table>
        
        <h4>Rates</h4>
        <table style="margin-bottom: 20px;">
            <tr><th>Metric</th><th>Rate</th><th>Count</th></tr>
            {{range $name, $summary := .MetricsSummaries}}
                {{if eq $summary.Type "rate"}}
                <tr><td>{{$name}}</td><td>{{printf "%.2f%%" (mul $summary.Average 100.0)}}</td><td>{{$summary.Count}}</td></tr>
                {{end}}
            {{end}}
        </table>
        
        <h4>Gauges</h4>
        <table style="margin-bottom: 20px;">
            <tr><th>Metric</th><th>Average</th><th>Latest</th></tr>
            {{range $name, $summary := .MetricsSummaries}}
                {{if eq $summary.Type "gauge"}}
                <tr><td>{{$name}}</td><td>{{printf "%.0f" $summary.Average}}</td><td>{{printf "%.0f" $summary.LatestValue}}</td></tr>
                {{end}}
            {{end}}
        </table>
    </div>
    {{end}}

    <h3>Request Details</h3>
    <table>
        <tr>
            <th>#</th>
            <th>Name</th>
            <th>Method</th>
            <th>URL</th>
            <th>Status</th>
            <th>Response Time</th>
            <th>Checks</th>
            <th>Timestamp</th>
        </tr>
        {{range $index, $req := .RequestDetails}}
        <tr>
            <td>{{add $index 1}}</td>
            <td>{{$req.Name}}</td>
            <td>{{$req.Verb}}</td>
            <td>{{$req.URL}}</td>
            <td>{{if $req.Success}}<span class="success">{{$req.StatusCode}} OK</span>{{else}}<span class="error">{{$req.Error}}</span>{{end}}</td>
            <td>{{$req.ResponseTime}}</td>
            <td>
                {{if gt (len $req.Checks) 0}}
                    {{range $checkIndex, $check := $req.Checks}}
                        {{if gt $checkIndex 0}}<br>{{end}}
                        {{if $check.Success}}
                            <span class="success">✓ {{$check.Name}}</span>
                        {{else}}
                            <span class="error">✗ {{$check.Name}}: {{$check.FailureMessage}}</span>
                        {{end}}
                    {{end}}
                {{else}}
                    -
                {{end}}
            </td>
            <td>{{$req.Timestamp.Format "2006-01-02 15:04:05"}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>`

func (f *HTMLFormatter) Format(report *Report) (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b float64) float64 { return a * b },
	}).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	// Prepare template data
	data := struct {
		*Report
		SuccessRate      float64
		CheckSuccessRate float64
		TotalDuration    time.Duration
	}{
		Report:        report,
		TotalDuration: report.EndTime.Sub(report.StartTime),
	}

	if report.TotalRequests > 0 {
		data.SuccessRate = float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
	}

	if report.TotalChecks > 0 {
		data.CheckSuccessRate = float64(report.SuccessfulChecks) / float64(report.TotalChecks) * 100
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// CSVFormatter formats reports as CSV
type CSVFormatter struct{}

func (f *CSVFormatter) Format(report *Report) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"Index", "Name", "Method", "URL", "Success", "StatusCode", "ResponseTime", "Error", "CheckFailures", "Timestamp"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// Write data
	for i, req := range report.RequestDetails {
		// Collect failed check messages
		var failedChecks []string
		for _, check := range req.Checks {
			if !check.Success && check.FailureMessage != "" {
				failedChecks = append(failedChecks, check.FailureMessage)
			}
		}
		checkFailuresStr := strings.Join(failedChecks, "; ")

		record := []string{
			strconv.Itoa(i + 1),
			req.Name,
			req.Verb,
			req.URL,
			strconv.FormatBool(req.Success),
			strconv.Itoa(req.StatusCode),
			req.ResponseTime.String(),
			req.Error,
			checkFailuresStr,
			req.Timestamp.Format(time.RFC3339),
		}
		if err := writer.Write(record); err != nil {
			return "", err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// JSONFormatter formats reports as JSON
type JSONFormatter struct{}

func (f *JSONFormatter) Format(report *Report) (string, error) {
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
			"averageResponseTime": report.AverageResponseTime.String(),
			"minResponseTime":     report.MinResponseTime.String(),
			"maxResponseTime":     report.MaxResponseTime.String(),
			"totalDuration":       report.EndTime.Sub(report.StartTime).String(),
			"startTime":           report.StartTime.Format(time.RFC3339),
			"endTime":             report.EndTime.Format(time.RFC3339),
		},
		"responseTimeDistribution": report.ResponseTimeDistribution,
		"errorBreakdown":           report.ErrorBreakdown,
		"checkSummaries":           report.CheckSummaries,
		"metricsSummaries":         report.MetricsSummaries,
		"requestDetails":           report.RequestDetails,
	}

	if report.TotalRequests > 0 {
		output["summary"].(map[string]interface{})["successRate"] = float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
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

// GetFormatter returns the appropriate formatter for the given format
func GetFormatter(format ReportFormat) Formatter {
	switch format {
	case FormatHTML:
		return &HTMLFormatter{}
	case FormatCSV:
		return &CSVFormatter{}
	case FormatJSON:
		return &JSONFormatter{}
	default:
		return &ConsoleFormatter{}
	}
}

// WriteReportToFile writes a formatted report to a file
func WriteReportToFile(report *Report, format ReportFormat, filename string) error {
	formatter := GetFormatter(format)
	content, err := formatter.Format(report)
	if err != nil {
		return err
	}

	return writeStringToFile(content, filename)
}

func writeStringToFile(content, filename string) error {
	return nil // This would typically write to a file, but we'll implement it in the main function
}

// HierarchicalFormatter formats hierarchical reports based on detail level
type HierarchicalFormatter struct {
	DetailLevel ReportDetailLevel
	Format      ReportFormat
}

func (f *HierarchicalFormatter) FormatHierarchical(report *HierarchicalReport) (string, error) {
	switch f.Format {
	case FormatConsole:
		return f.formatHierarchicalConsole(report)
	case FormatHTML:
		return f.formatHierarchicalHTML(report)
	case FormatJSON:
		return f.formatHierarchicalJSON(report)
	default:
		// Fallback to regular console format
		return f.formatHierarchicalConsole(report)
	}
}

func (f *HierarchicalFormatter) formatHierarchicalConsole(report *HierarchicalReport) (string, error) {
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

	// Show goroutine details if requested
	if f.DetailLevel == DetailVirtualuser || f.DetailLevel == DetailIteration || f.DetailLevel == DetailFull {
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
	if f.DetailLevel == DetailIteration || f.DetailLevel == DetailFull {
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
	if f.DetailLevel == DetailFull {
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

func (f *HierarchicalFormatter) formatHierarchicalJSON(report *HierarchicalReport) (string, error) {
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

	if f.DetailLevel == DetailVirtualuser || f.DetailLevel == DetailIteration || f.DetailLevel == DetailFull {
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

			if f.DetailLevel == DetailIteration || f.DetailLevel == DetailFull {
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

					if f.DetailLevel == DetailFull {
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

func (f *HierarchicalFormatter) formatHierarchicalHTML(report *HierarchicalReport) (string, error) {
	const hierarchicalHTMLTemplate = `<!DOCTYPE html>
<html>
<head>
    <title>HTTP Request Report - Hierarchical</title>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            margin: 20px; 
            background-color: #f9f9f9;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            border-radius: 10px;
            margin-bottom: 20px;
        }
        .summary { 
            background: white; 
            padding: 20px; 
            border-radius: 8px; 
            margin-bottom: 20px; 
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .metric { 
            display: inline-block; 
            margin-right: 30px; 
            padding: 10px;
            background: #f8f9fa;
            border-radius: 5px;
            border-left: 4px solid #007bff;
        }
        .metric strong { color: #495057; }
        .success { color: #28a745; font-weight: bold; }
        .error { color: #dc3545; font-weight: bold; }
        .warning { color: #ffc107; font-weight: bold; }
        
        /* Collapsible sections */
        .collapsible {
            background-color: #007bff;
            color: white;
            cursor: pointer;
            padding: 15px;
            width: 100%;
            border: none;
            text-align: left;
            outline: none;
            font-size: 16px;
            font-weight: bold;
            border-radius: 5px;
            margin: 5px 0;
            transition: background-color 0.3s;
        }
        .collapsible:hover { background-color: #0056b3; }
        .collapsible.active { background-color: #0056b3; }
        
        .content {
            padding: 0;
            display: none;
            overflow: hidden;
            background-color: white;
            border-radius: 0 0 5px 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .content.show { display: block; padding: 15px; }
        
        .goroutine-header {
            background-color: #6c757d;
            color: white;
            padding: 10px 15px;
            border-radius: 5px;
            margin: 10px 0 5px 0;
        }
        .iteration-header {
            background-color: #17a2b8;
            color: white;
            padding: 8px 12px;
            border-radius: 4px;
            margin: 8px 0 5px 20px;
        }
        
        table { 
            border-collapse: collapse; 
            width: 100%; 
            margin: 10px 0;
            background: white;
            border-radius: 5px;
            overflow: hidden;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        th, td { 
            border: 1px solid #dee2e6; 
            padding: 12px 8px; 
            text-align: left; 
        }
        th { 
            background-color: #e9ecef; 
            font-weight: bold;
            color: #495057;
        }
        tr:nth-child(even) { background-color: #f8f9fa; }
        tr:hover { background-color: #e8f4f8; }
        
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin: 15px 0;
        }
        .stat-card {
            background: white;
            padding: 15px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            border-left: 4px solid #007bff;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #007bff;
        }
        .stat-label {
            font-size: 12px;
            color: #6c757d;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        
        .progress-bar {
            width: 100%;
            height: 8px;
            background-color: #e9ecef;
            border-radius: 4px;
            overflow: hidden;
            margin: 5px 0;
        }
        .progress-fill {
            height: 100%;
            background-color: #28a745;
            transition: width 0.3s ease;
        }
        
        .request-details {
            margin: 20px 0;
        }
        .url { 
            max-width: 300px; 
            overflow: hidden; 
            text-overflow: ellipsis; 
            white-space: nowrap;
        }
        .timestamp {
            font-size: 12px;
            color: #6c757d;
        }
    </style>
    <script>
        function toggleCollapsible(element) {
            element.classList.toggle("active");
            var content = element.nextElementSibling;
            if (content.classList.contains("show")) {
                content.classList.remove("show");
            } else {
                content.classList.add("show");
            }
        }
        
        function expandAll() {
            var collapsibles = document.getElementsByClassName("collapsible");
            var contents = document.getElementsByClassName("content");
            for (var i = 0; i < collapsibles.length; i++) {
                collapsibles[i].classList.add("active");
                contents[i].classList.add("show");
            }
        }
        
        function collapseAll() {
            var collapsibles = document.getElementsByClassName("collapsible");
            var contents = document.getElementsByClassName("content");
            for (var i = 0; i < collapsibles.length; i++) {
                collapsibles[i].classList.remove("active");
                contents[i].classList.remove("show");
            }
        }
    </script>
</head>
<body>
    <div class="header">
        <h1>🚀 HTTP Request Report - Hierarchical View</h1>
        <p>Generated on {{.Summary.StartTime.Format "2006-01-02 15:04:05"}}</p>
    </div>
    
    <div style="margin-bottom: 15px;">
        <button onclick="expandAll()" style="background:#28a745;color:white;border:none;padding:8px 15px;border-radius:4px;cursor:pointer;margin-right:10px;">Expand All</button>
        <button onclick="collapseAll()" style="background:#6c757d;color:white;border:none;padding:8px 15px;border-radius:4px;cursor:pointer;">Collapse All</button>
    </div>
    
    <div class="summary">
        <h2>📊 Overall Summary</h2>
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">{{.TotalVirtualUsers}}</div>
                <div class="stat-label">Total VirtualUserReports</div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: 100%;"></div>
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{{.SuccessfulVirtualUsers}}</div>
                <div class="stat-label">Successful VirtualUserReports</div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: {{if gt .TotalVirtualUsers 0}}{{printf "%.0f" (mul (div (float64 .SuccessfulVirtualUsers) (float64 .TotalVirtualUsers)) 100.0)}}{{else}}0{{end}}%;"></div>
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.Summary.TotalRequests}}</div>
                <div class="stat-label">Total Requests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{{.Summary.SuccessfulRequests}}</div>
                <div class="stat-label">Successful Requests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value error">{{.Summary.FailedRequests}}</div>
                <div class="stat-label">Failed Requests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{.Summary.AverageResponseTime}}</div>
                <div class="stat-label">Avg Response Time</div>
            </div>
        </div>
        
        <div style="margin-top: 20px;">
            <div class="metric">
                <strong>Success Rate:</strong> 
                <span class="{{if ge .SuccessRate 90.0}}success{{else if ge .SuccessRate 70.0}}warning{{else}}error{{end}}">
                    {{printf "%.1f" .SuccessRate}}%
                </span>
            </div>
            <div class="metric"><strong>Total Duration:</strong> {{.TotalDuration}}</div>
            <div class="metric"><strong>Min Response:</strong> {{.Summary.MinResponseTime}}</div>
            <div class="metric"><strong>Max Response:</strong> {{.Summary.MaxResponseTime}}</div>
        </div>
    </div>

    {{if gt .Summary.TotalChecks 0}}
    <div class="summary">
        <h2>✅ Check Results</h2>
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">{{.Summary.TotalChecks}}</div>
                <div class="stat-label">Total Checks</div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{{.Summary.SuccessfulChecks}}</div>
                <div class="stat-label">Successful Checks</div>
            </div>
            <div class="stat-card">
                <div class="stat-value error">{{.Summary.FailedChecks}}</div>
                <div class="stat-label">Failed Checks</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{if gt .Summary.TotalChecks 0}}{{printf "%.1f" (mul (div (float64 .Summary.SuccessfulChecks) (float64 .Summary.TotalChecks)) 100.0)}}%{{else}}N/A{{end}}</div>
                <div class="stat-label">Check Success Rate</div>
            </div>
        </div>
        
        <button class="collapsible" onclick="toggleCollapsible(this)" style="background-color: #28a745; margin: 15px 0;">
            📋 Check Breakdown
        </button>
        <div class="content">
            {{range $name, $summary := .Summary.CheckSummaries}}
            <div style="padding: 10px; border-left: 4px solid {{if gt $summary.FailedRuns 0}}#dc3545{{else}}#28a745{{end}}; margin: 5px 0; background: #f8f9fa;">
                <strong>{{$name}}</strong><br>
                <small>{{$summary.TotalRuns}} runs ({{$summary.SuccessfulRuns}} successful, {{$summary.FailedRuns}} failed)</small>
                {{if gt (len $summary.FailureMessages) 0}}
                <br><small style="color: #dc3545;"><strong>Failures:</strong> {{range $i, $msg := $summary.FailureMessages}}{{if $i}}, {{end}}{{$msg}}{{end}}</small>
                {{end}}
            </div>
            {{end}}
        </div>
    </div>
    {{end}}

    {{if or (eq .DetailLevel "virtualuser") (eq .DetailLevel "iteration") (eq .DetailLevel "full")}}
    <button class="collapsible" onclick="toggleCollapsible(this)">
        🔧 Goroutine Breakdown ({{len .VirtualUserReports}} goroutines)
    </button>
    <div class="content">
        {{range $goroutineIndex, $goroutine := .VirtualUserReports}}
        <div class="goroutine-header">
            <strong>VirtualUser {{$goroutine.GoroutineID}}</strong>
            - {{$goroutine.TotalRequests}} requests 
            - {{printf "%.1f" (mul (div (float64 $goroutine.SuccessfulRequests) (float64 $goroutine.TotalRequests)) 100.0)}}% success rate
            - Avg: {{$goroutine.AverageResponseTime}}
        </div>
        
        <div style="margin-left: 20px;">
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-value">{{$goroutine.TotalIterations}}</div>
                    <div class="stat-label">Iterations</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value success">{{$goroutine.SuccessfulIterations}}</div>
                    <div class="stat-label">Successful</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value error">{{$goroutine.FailedIterations}}</div>
                    <div class="stat-label">Failed</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{{$goroutine.TotalDuration}}</div>
                    <div class="stat-label">Duration</div>
                </div>
            </div>
            
            {{if or (eq $.DetailLevel "iteration") (eq $.DetailLevel "full")}}
            {{if gt (len $goroutine.Iterations) 0}}
            <button class="collapsible" onclick="toggleCollapsible(this)" style="background-color: #17a2b8; margin-left: 0;">
                📋 Iteration Details ({{len $goroutine.Iterations}} iterations)
            </button>
            <div class="content">
                {{range $iterationIndex, $iteration := $goroutine.Iterations}}
                <div class="iteration-header">
                    <strong>Iteration {{$iteration.IterationID}}</strong>
                    - {{$iteration.TotalRequests}} requests
                    - {{if gt $iteration.TotalRequests 0}}{{printf "%.1f" (mul (div (float64 $iteration.SuccessfulRequests) (float64 $iteration.TotalRequests)) 100.0)}}%{{else}}0%{{end}} success
                    - {{$iteration.AverageResponseTime}} avg
                    - {{$iteration.TotalDuration}} duration
                </div>
                
                {{if eq $.DetailLevel "full"}}
                {{if gt (len $iteration.RequestResults) 0}}
                <div style="margin-left: 40px;">
                    <table>
                        <tr>
                            <th>#</th>
                            <th>Name</th>
                            <th>Method</th>
                            <th>URL</th>
                            <th>Status</th>
                            <th>Response Time</th>
                            <th>Checks</th>
                            <th>Timestamp</th>
                        </tr>
                        {{range $requestIndex, $request := $iteration.RequestResults}}
                        <tr>
                            <td>{{add $requestIndex 1}}</td>
                            <td>{{$request.Name}}</td>
                            <td><strong>{{$request.Verb}}</strong></td>
                            <td class="url" title="{{$request.URL}}">{{$request.URL}}</td>
                            <td>
                                {{if $request.Success}}
                                    <span class="success">{{$request.StatusCode}} ✓</span>
                                {{else}}
                                    <span class="error">{{$request.Error}} ✗</span>
                                {{end}}
                            </td>
                            <td>{{$request.ResponseTime}}</td>
                            <td>
                                {{if gt (len $request.Checks) 0}}
                                    {{range $checkIndex, $check := $request.Checks}}
                                        {{if gt $checkIndex 0}}<br>{{end}}
                                        {{if $check.Success}}
                                            <span class="success">✓ {{$check.Name}}</span>
                                        {{else}}
                                            <span class="error">✗ {{$check.Name}}: {{$check.FailureMessage}}</span>
                                        {{end}}
                                    {{end}}
                                {{else}}
                                    -
                                {{end}}
                            </td>
                            <td class="timestamp">{{$request.Timestamp.Format "15:04:05.000"}}</td>
                        </tr>
                        {{end}}
                    </table>
                </div>
                {{end}}
                {{end}}
                {{end}}
            </div>
            {{end}}
            {{end}}
        </div>
        {{end}}
    </div>
    {{end}}

    <div style="margin-top: 30px; padding: 20px; background: #f8f9fa; border-radius: 8px; text-align: center;">
        <small style="color: #6c757d;">
            🎯 Tip: Click on section headers to expand/collapse details. 
            Use "Expand All" / "Collapse All" buttons for bulk operations.
        </small>
    </div>
</body>
</html>`

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
		"collectFailedChecks": func(requests []RequestResult) string {
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
		*HierarchicalReport
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
