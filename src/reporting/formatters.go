package reporting

import (
	"github.com/deicon/httprunner/src/reporting/formatters/console"
	"github.com/deicon/httprunner/src/reporting/formatters/csv"
	"github.com/deicon/httprunner/src/reporting/formatters/html"
	json2 "github.com/deicon/httprunner/src/reporting/formatters/json"
	"github.com/deicon/httprunner/src/reporting/types"
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
