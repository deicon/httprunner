package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/deicon/httprunner/metrics"
	"github.com/deicon/httprunner/reporting/formatters/console"
	"github.com/deicon/httprunner/reporting/formatters/csv"
	"github.com/deicon/httprunner/reporting/formatters/html"
	json2 "github.com/deicon/httprunner/reporting/formatters/json"
	"github.com/deicon/httprunner/reporting/types"
)

// Formatter interface for different report formats
type Formatter interface {
	Format(report *types.Report) (string, error)
}

// HTMLFormatter.Format moved to formatter_html.go

// JSON formatter moved

// GetFormatter returns the appropriate formatter for the given format
func GetFormatter(format types.ReportFormat) Formatter {
	switch format {
	case types.FormatHTML:
		return &html.HTMLFormatter{}
	case types.FormatCSV:
		return &csv.CSVFormatter{}
	case types.FormatJSON:
		return &json2.JSONFormatter{}
	default:
		return &console.ConsoleFormatter{}
	}
}

// WriteReportToFile writes a formatted report to a file
func WriteReportToFile(report *types.Report, format types.ReportFormat, filename string) error {
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

	// Show goroutine details if requested
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

func (f *HierarchicalFormatter) formatHierarchicalHTML(report *types.HierarchicalReport) (string, error) {
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
        🔧 VirtualUser Breakdown ({{len .VirtualUserReports}} goroutines)
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
