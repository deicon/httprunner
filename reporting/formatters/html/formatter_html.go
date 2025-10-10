package html

import (
	"bytes"
	_ "embed"
	"html/template"
	"strings"
	"time"

	"github.com/deicon/httprunner/reporting/types"
)

// HTMLFormatter formats reports as HTML
type HTMLFormatter struct{}

//go:embed templates/htmlTemplate.html
var htmlTemplate string

func (f *HTMLFormatter) Format(report *types.Report) (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"add":         func(a, b int) int { return a + b },
		"mul":         func(a, b float64) float64 { return a * b },
		"stripSuffix": func(s, suffix string) string { return strings.TrimSuffix(s, suffix) },
	}).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	// Prepare template data
	data := struct {
		*types.Report
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
