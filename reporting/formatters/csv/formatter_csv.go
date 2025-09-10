package csv

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/deicon/httprunner/reporting/types"
)

// CSVFormatter formats reports as CSV
type CSVFormatter struct{}

func (f *CSVFormatter) Format(report *types.Report) (string, error) {
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
