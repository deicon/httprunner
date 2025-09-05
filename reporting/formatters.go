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
			"totalRequests":         report.TotalRequests,
			"successfulRequests":    report.SuccessfulRequests,
			"failedRequests":        report.FailedRequests,
			"successRate":           0.0,
			"averageResponseTime":   report.AverageResponseTime.String(),
			"minResponseTime":       report.MinResponseTime.String(),
			"maxResponseTime":       report.MaxResponseTime.String(),
			"totalDuration":         report.EndTime.Sub(report.StartTime).String(),
			"startTime":             report.StartTime.Format(time.RFC3339),
			"endTime":               report.EndTime.Format(time.RFC3339),
		},
		"responseTimeDistribution": report.ResponseTimeDistribution,
		"errorBreakdown":          report.ErrorBreakdown,
		"requestDetails":          report.RequestDetails,
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