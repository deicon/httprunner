package console

import (
	"github.com/deicon/httprunner/src/reporting/types"
	"strings"
	"testing"
	"time"
)

func TestConsoleFormatterBasic(t *testing.T) {
	f := &ConsoleFormatter{}
	rep := &types.Report{TotalRequests: 1, SuccessfulRequests: 1, StartTime: time.Now(), EndTime: time.Now().Add(time.Second)}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if !strings.Contains(out, "HTTP Request Report") {
		t.Fatalf("missing title: %s", out)
	}
}
