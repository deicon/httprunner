package csv

import (
	"github.com/deicon/httprunner/src/reporting/types"
	"strings"
	"testing"
	"time"
)

func TestCSVFormatterBasic(t *testing.T) {
	f := &CSVFormatter{}
	rep := &types.Report{RequestDetails: []types.RequestResult{{Name: "A", Verb: "GET", URL: "/", Success: true, StatusCode: 200, ResponseTime: 10 * time.Millisecond, Timestamp: time.Now()}}, StartTime: time.Now(), EndTime: time.Now().Add(time.Second)}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.HasPrefix(out, "Index,Name,Method,URL,Success,StatusCode,ResponseTime,Error,CheckFailures,Timestamp\n") {
		t.Fatalf("header missing: %s", out)
	}
}
