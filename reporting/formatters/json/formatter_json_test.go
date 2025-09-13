package json

import (
	"encoding/json"
	"github.com/deicon/httprunner/reporting/types"
	"testing"
	"time"
)

func TestJSONFormatterBasic(t *testing.T) {
	f := &JSONFormatter{}
	rep := &types.Report{TotalRequests: 2, SuccessfulRequests: 2, StartTime: time.Now(), EndTime: time.Now().Add(time.Second)}
	out, err := f.Format(rep)
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}
