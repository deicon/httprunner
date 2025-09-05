package reporting

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"strconv"
	"time"
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

	// Request details (limited to first 10 for console)
	if len(report.RequestDetails) > 0 {
		buf.WriteString("Request Details (showing first 10):\n")
		limit := 10
		if len(report.RequestDetails) < limit {
			limit = len(report.RequestDetails)
		}

		for i := 0; i < limit; i++ {
			req := report.RequestDetails[i]
			status := "SUCCESS"
			if !req.Success {
				status = fmt.Sprintf("ERROR: %s", req.Error)
			}
			buf.WriteString(fmt.Sprintf("%d. %s %s - %s - %v\n",
				i+1, req.Verb, req.URL, status, req.ResponseTime))
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

    <h3>Request Details</h3>
    <table>
        <tr>
            <th>#</th>
            <th>Method</th>
            <th>URL</th>
            <th>Status</th>
            <th>Response Time</th>
            <th>Timestamp</th>
        </tr>
        {{range $index, $req := .RequestDetails}}
        <tr>
            <td>{{add $index 1}}</td>
            <td>{{$req.Verb}}</td>
            <td>{{$req.URL}}</td>
            <td>{{if $req.Success}}<span class="success">{{$req.StatusCode}} OK</span>{{else}}<span class="error">{{$req.Error}}</span>{{end}}</td>
            <td>{{$req.ResponseTime}}</td>
            <td>{{$req.Timestamp.Format "2006-01-02 15:04:05"}}</td>
        </tr>
        {{end}}
    </table>
</body>
</html>`

func (f *HTMLFormatter) Format(report *Report) (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	// Prepare template data
	data := struct {
		*Report
		SuccessRate   float64
		TotalDuration time.Duration
	}{
		Report:        report,
		TotalDuration: report.EndTime.Sub(report.StartTime),
	}

	if report.TotalRequests > 0 {
		data.SuccessRate = float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
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
	header := []string{"Index", "Name", "Method", "URL", "Success", "StatusCode", "ResponseTime", "Error", "Timestamp"}
	if err := writer.Write(header); err != nil {
		return "", err
	}

	// Write data
	for i, req := range report.RequestDetails {
		record := []string{
			strconv.Itoa(i + 1),
			req.Name,
			req.Verb,
			req.URL,
			strconv.FormatBool(req.Success),
			strconv.Itoa(req.StatusCode),
			req.ResponseTime.String(),
			req.Error,
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
			"averageResponseTime": report.AverageResponseTime.String(),
			"minResponseTime":     report.MinResponseTime.String(),
			"maxResponseTime":     report.MaxResponseTime.String(),
			"totalDuration":       report.EndTime.Sub(report.StartTime).String(),
			"startTime":           report.StartTime.Format(time.RFC3339),
			"endTime":             report.EndTime.Format(time.RFC3339),
		},
		"responseTimeDistribution": report.ResponseTimeDistribution,
		"errorBreakdown":           report.ErrorBreakdown,
		"requestDetails":           report.RequestDetails,
	}

	if report.TotalRequests > 0 {
		output["summary"].(map[string]interface{})["successRate"] = float64(report.SuccessfulRequests) / float64(report.TotalRequests) * 100
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

	buf.WriteString(fmt.Sprintf("Total Goroutines: %d\n", report.TotalGoroutines))
	buf.WriteString(fmt.Sprintf("Successful Goroutines: %d\n", report.SuccessfulGoroutines))
	buf.WriteString(fmt.Sprintf("Failed Goroutines: %d\n", report.FailedGoroutines))
	if report.TotalGoroutines > 0 {
		successRate := float64(report.SuccessfulGoroutines) / float64(report.TotalGoroutines) * 100
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

	// Show goroutine details if requested
	if f.DetailLevel == DetailGoroutine || f.DetailLevel == DetailIteration || f.DetailLevel == DetailFull {
		buf.WriteString("Goroutine Breakdown\n")
		buf.WriteString("===================\n\n")

		for _, gr := range report.Goroutines {
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

		for _, gr := range report.Goroutines {
			if len(gr.Iterations) > 0 {
				buf.WriteString(fmt.Sprintf("Goroutine %d Iterations:\n", gr.GoroutineID))
				for _, iter := range gr.Iterations {
					buf.WriteString(fmt.Sprintf("  Iteration %d:\n", iter.IterationID))
					buf.WriteString(fmt.Sprintf("    Requests: %d (Success: %d, Failed: %d)\n",
						iter.TotalRequests, iter.SuccessfulRequests, iter.FailedRequests))
					buf.WriteString(fmt.Sprintf("    Average Response Time: %v\n", iter.AverageResponseTime))
					buf.WriteString(fmt.Sprintf("    Duration: %v\n", iter.TotalDuration))
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
		for _, gr := range report.Goroutines {
			for _, iter := range gr.Iterations {
				if len(iter.RequestResults) > 0 {
					buf.WriteString(fmt.Sprintf("Goroutine %d, Iteration %d:\n", gr.GoroutineID, iter.IterationID))
					for _, req := range iter.RequestResults {
						status := "SUCCESS"
						if !req.Success {
							status = fmt.Sprintf("ERROR: %s", req.Error)
						}
						buf.WriteString(fmt.Sprintf("  %d. %s %s - %s - %v\n",
							count, req.Verb, req.URL, status, req.ResponseTime))
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
			"totalGoroutines":      report.TotalGoroutines,
			"successfulGoroutines": report.SuccessfulGoroutines,
			"failedGoroutines":     report.FailedGoroutines,
			"totalRequests":        report.Summary.TotalRequests,
			"successfulRequests":   report.Summary.SuccessfulRequests,
			"failedRequests":       report.Summary.FailedRequests,
			"averageResponseTime":  report.Summary.AverageResponseTime.String(),
			"totalDuration":        report.Summary.EndTime.Sub(report.Summary.StartTime).String(),
			"startTime":            report.Summary.StartTime.Format(time.RFC3339),
			"endTime":              report.Summary.EndTime.Format(time.RFC3339),
		},
	}

	if f.DetailLevel == DetailGoroutine || f.DetailLevel == DetailIteration || f.DetailLevel == DetailFull {
		goroutines := make([]map[string]interface{}, 0, len(report.Goroutines))
		for _, gr := range report.Goroutines {
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
                <div class="stat-value">{{.TotalGoroutines}}</div>
                <div class="stat-label">Total Goroutines</div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: 100%;"></div>
                </div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{{.SuccessfulGoroutines}}</div>
                <div class="stat-label">Successful Goroutines</div>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: {{if gt .TotalGoroutines 0}}{{printf "%.0f" (mul (div (float64 .SuccessfulGoroutines) (float64 .TotalGoroutines)) 100.0)}}{{else}}0{{end}}%;"></div>
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

    {{if or (eq .DetailLevel "goroutine") (eq .DetailLevel "iteration") (eq .DetailLevel "full")}}
    <button class="collapsible" onclick="toggleCollapsible(this)">
        🔧 Goroutine Breakdown ({{len .Goroutines}} goroutines)
    </button>
    <div class="content">
        {{range $goroutineIndex, $goroutine := .Goroutines}}
        <div class="goroutine-header">
            <strong>Goroutine {{$goroutine.GoroutineID}}</strong>
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
