package metrics

import (
	"testing"
	"time"
)

func TestMetricBasicOperations(t *testing.T) {
	m := NewMetric("trend_metric", Trend)
	m.AddValue(1, nil)
	m.AddValue(2, nil)
	m.AddValue(3, nil)

	if got := m.GetCount(); got != 3 {
		t.Fatalf("GetCount=%d want 3", got)
	}
	if got := m.GetSum(); got != 6 {
		t.Fatalf("GetSum=%v want 6", got)
	}
	if got := m.GetAverage(); got != 2 {
		t.Fatalf("GetAverage=%v want 2", got)
	}
	if got := m.GetMin(); got != 1 {
		t.Fatalf("GetMin=%v want 1", got)
	}
	if got := m.GetMax(); got != 3 {
		t.Fatalf("GetMax=%v want 3", got)
	}
	if got := m.GetPercentile(50); got != 2 {
		t.Fatalf("p50=%v want 2", got)
	}
	if got := m.GetPercentile(90); got != 2 { // index math truncates to 1 for 3 values
		t.Fatalf("p90=%v want 2", got)
	}

	latest := m.GetLatest()
	if latest == nil || latest.Value != 3 {
		t.Fatalf("GetLatest unexpected: %#v", latest)
	}

	// Empty metric percentile returns 0
	empty := NewMetric("empty", Trend)
	if got := empty.GetPercentile(50); got != 0 {
		t.Fatalf("empty p50=%v want 0", got)
	}
}

func TestMetricsCollectorInitialization(t *testing.T) {
	mc := NewMetricsCollector()
	// Expected standard metrics
	expected := map[string]MetricType{
		"checks":                   Rate,
		"data_received":            Counter,
		"data_sent":                Counter,
		"dropped_iterations":       Counter,
		"iteration_duration":       Trend,
		"iterations":               Counter,
		"vus":                      Gauge,
		"vus_max":                  Gauge,
		"http_req_blocked":         Trend,
		"http_req_connecting":      Trend,
		"http_req_duration":        Trend,
		"http_req_failed":          Rate,
		"http_req_receiving":       Trend,
		"http_req_sending":         Trend,
		"http_req_tls_handshaking": Trend,
		"http_req_waiting":         Trend,
		"http_reqs":                Counter,
	}
	for name, typ := range expected {
		m := mc.GetMetric(name)
		if m == nil || m.Type != typ {
			t.Fatalf("metric %s type=%v want %v (exists=%v)", name, func() MetricType {
				if m != nil {
					return m.Type
				}
				return ""
			}(), typ, m != nil)
		}
	}
}

func TestRecordValueAndGetAll(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordValue("http_reqs", 1, map[string]string{"k": "v"})
	m := mc.GetMetric("http_reqs")
	if m.GetCount() != 1 || m.GetSum() != 1 {
		t.Fatalf("http_reqs count/sum mismatch: %d/%v", m.GetCount(), m.GetSum())
	}
	all := mc.GetAllMetrics()
	if _, ok := all["http_reqs"]; !ok {
		t.Fatalf("GetAllMetrics missing http_reqs")
	}
}

func TestRecordCheck(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordCheck(true, nil)
	checks := mc.GetMetric("checks")
	if checks.GetSum() <= 0 {
		t.Fatalf("checks sum should be > 0")
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	mc := NewMetricsCollector()
	dur := 150 * time.Millisecond
	mc.RecordHTTPRequest(dur, 10*time.Millisecond, 20*time.Millisecond, 30*time.Millisecond, 40*time.Millisecond, 50*time.Millisecond, 5*time.Millisecond, false, 100, 200, map[string]string{"url": "/t"})

	if mc.GetMetric("http_reqs").GetSum() != 1 {
		t.Fatalf("http_reqs sum should be 1")
	}
	if mc.GetMetric("http_req_duration").GetLatest().Value != float64(dur.Milliseconds()) {
		t.Fatalf("http_req_duration latest mismatch")
	}
	if mc.GetMetric("http_req_failed").GetLatest().Value != 0 {
		t.Fatalf("http_req_failed should be 0 for success")
	}

	// failed request
	mc.RecordHTTPRequest(dur, 0, 0, 0, 0, 0, 0, true, 0, 0, nil)
	if mc.GetMetric("http_req_failed").GetLatest().Value != 1 {
		t.Fatalf("http_req_failed should be 1 for failure")
	}
}

func TestRecordIterationAndUsers(t *testing.T) {
	mc := NewMetricsCollector()
	mc.RecordIteration(25*time.Millisecond, map[string]string{"iter": "0"})
	mc.UpdateVirtualUsers(3, 5)

	if mc.GetMetric("iterations").GetSum() != 1 {
		t.Fatalf("iterations sum should be 1")
	}
	if mc.GetMetric("iteration_duration").GetCount() != 1 {
		t.Fatalf("iteration_duration count should be 1")
	}
	if mc.GetMetric("vus").GetLatest().Value != 3 || mc.GetMetric("vus_max").GetLatest().Value != 5 {
		t.Fatalf("vus/vus_max latest values mismatch")
	}
}

func TestMetricGetSummaryVariants(t *testing.T) {
	// Counter
	c := NewMetric("counter", Counter)
	c.AddValue(1, nil)
	c.AddValue(2, nil)
	cs := c.GetSummary()
	if cs.Sum != 3 || cs.Count != 2 {
		t.Fatalf("counter summary mismatch: %+v", cs)
	}

	// Gauge
	g := NewMetric("gauge", Gauge)
	g.AddValue(5, nil)
	g.AddValue(7, nil)
	gs := g.GetSummary()
	if gs.LatestValue != 7 || gs.Average != 7 {
		t.Fatalf("gauge summary mismatch: %+v", gs)
	}

	// Rate
	r := NewMetric("rate", Rate)
	r.AddValue(0.1, nil)
	r.AddValue(0.3, nil)
	rs := r.GetSummary()
	if rs.Average <= 0 || rs.Sum <= 0 {
		t.Fatalf("rate summary mismatch: %+v", rs)
	}

	// Trend
	tnd := NewMetric("trend", Trend)
	for _, v := range []float64{10, 20, 30, 40} {
		tnd.AddValue(v, nil)
	}
	ts := tnd.GetSummary()
	if ts.Min != 10 || ts.Max != 40 || ts.P50 == 0 || ts.P90 == 0 || ts.P95 == 0 || ts.P99 == 0 {
		t.Fatalf("trend summary fields missing: %+v", ts)
	}
}

func TestGetSummariesAndRates(t *testing.T) {
	mc := NewMetricsCollector()
	// Ensure runtime > 0 before SetEndTime
	mc.RecordHTTPRequest(10*time.Millisecond, 0, 0, 0, 0, 0, 0, false, 100, 50, nil)
	time.Sleep(10 * time.Millisecond)
	mc.SetEndTime()

	sum := mc.GetSummaries()
	httpReqs := sum["http_reqs"]
	if httpReqs.Rate <= 0 || httpReqs.RateUnit != "req/s" {
		t.Fatalf("http_reqs rate missing/invalid: %+v", httpReqs)
	}
	dataSent := sum["data_sent"]
	if dataSent.Rate <= 0 || dataSent.RateUnit != "bytes/s" {
		t.Fatalf("data_sent rate invalid: %+v", dataSent)
	}
}

func TestPerVUMetricsAndPerIteration(t *testing.T) {
	mc := NewMetricsCollector()
	// 4 http_reqs and 300 bytes sent across 2 iterations
	mc.RecordHTTPRequest(1, 0, 0, 0, 0, 0, 0, false, 100, 0, nil)
	mc.RecordHTTPRequest(1, 0, 0, 0, 0, 0, 0, false, 200, 0, nil)
	mc.RecordIteration(2, nil)
	mc.RecordHTTPRequest(1, 0, 0, 0, 0, 0, 0, false, 0, 0, nil)
	mc.RecordHTTPRequest(1, 0, 0, 0, 0, 0, 0, false, 0, 0, nil)
	mc.RecordIteration(2, nil)

	perVU := mc.GetPerVUMetrics(2)
	if perVU["http_reqs_per_vu"] != 2 { // total 4 across 2 VUs
		t.Fatalf("http_reqs_per_vu expected 2, got %v", perVU["http_reqs_per_vu"])
	}

	perIter := mc.GetPerIterationMetrics()
	if perIter["data_sent_per_iter"] != 150 { // 300/2
		t.Fatalf("data_sent_per_iter expected 150, got %v", perIter["data_sent_per_iter"])
	}
}
