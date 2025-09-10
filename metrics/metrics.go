package metrics

import (
	"sync"
	"time"
)

// MetricType represents the type of metric (Counter, Gauge, Rate, Trend)
type MetricType string

const (
	Counter MetricType = "counter"
	Gauge   MetricType = "gauge"
	Rate    MetricType = "rate"
	Trend   MetricType = "trend"
)

// MetricValue represents a single metric measurement
type MetricValue struct {
	Timestamp time.Time
	Value     float64
	Tags      map[string]string
}

// Metric represents a metric with its type and collected values
type Metric struct {
	Name   string
	Type   MetricType
	Values []MetricValue
	mu     sync.RWMutex
}

// NewMetric creates a new metric
func NewMetric(name string, metricType MetricType) *Metric {
	return &Metric{
		Name:   name,
		Type:   metricType,
		Values: make([]MetricValue, 0),
	}
}

// AddValue adds a new value to the metric
func (m *Metric) AddValue(value float64, tags map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Values = append(m.Values, MetricValue{
		Timestamp: time.Now(),
		Value:     value,
		Tags:      tags,
	})
}

// GetValues returns a copy of all values
func (m *Metric) GetValues() []MetricValue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	values := make([]MetricValue, len(m.Values))
	copy(values, m.Values)
	return values
}

// GetLatest returns the most recent value
func (m *Metric) GetLatest() *MetricValue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Values) == 0 {
		return nil
	}
	return &m.Values[len(m.Values)-1]
}

// GetCount returns the total number of values (for counters)
func (m *Metric) GetCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.Values)
}

// GetSum returns the sum of all values
func (m *Metric) GetSum() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sum float64
	for _, v := range m.Values {
		sum += v.Value
	}
	return sum
}

// GetAverage returns the average of all values (for trends)
func (m *Metric) GetAverage() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range m.Values {
		sum += v.Value
	}
	return sum / float64(len(m.Values))
}

// GetMin returns the minimum value (for trends)
func (m *Metric) GetMin() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Values) == 0 {
		return 0
	}

	min := m.Values[0].Value
	for _, v := range m.Values[1:] {
		if v.Value < min {
			min = v.Value
		}
	}
	return min
}

// GetMax returns the maximum value (for trends)
func (m *Metric) GetMax() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Values) == 0 {
		return 0
	}

	max := m.Values[0].Value
	for _, v := range m.Values[1:] {
		if v.Value > max {
			max = v.Value
		}
	}
	return max
}

// GetPercentile returns the specified percentile (for trends)
func (m *Metric) GetPercentile(percentile float64) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.Values) == 0 {
		return 0
	}

	// Simple percentile calculation - sort values first
	values := make([]float64, len(m.Values))
	for i, v := range m.Values {
		values[i] = v.Value
	}

	// Basic bubble sort for simplicity
	for i := 0; i < len(values); i++ {
		for j := 0; j < len(values)-1-i; j++ {
			if values[j] > values[j+1] {
				values[j], values[j+1] = values[j+1], values[j]
			}
		}
	}

	index := int(percentile / 100.0 * float64(len(values)-1))
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

// MetricsCollector manages all metrics for a test run
type MetricsCollector struct {
	metrics   map[string]*Metric
	startTime time.Time
	endTime   time.Time
	mu        sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector with standard metrics
func NewMetricsCollector() *MetricsCollector {
	collector := &MetricsCollector{
		metrics:   make(map[string]*Metric),
		startTime: time.Now(),
	}

	// Initialize standard built-in metrics
	collector.registerMetric("checks", Rate)
	collector.registerMetric("data_received", Counter)
	collector.registerMetric("data_sent", Counter)
	collector.registerMetric("dropped_iterations", Counter)
	collector.registerMetric("iteration_duration", Trend)
	collector.registerMetric("iterations", Counter)
	collector.registerMetric("vus", Gauge)
	collector.registerMetric("vus_max", Gauge)

	// Initialize HTTP metrics
	collector.registerMetric("http_req_blocked", Trend)
	collector.registerMetric("http_req_connecting", Trend)
	collector.registerMetric("http_req_duration", Trend)
	collector.registerMetric("http_req_failed", Rate)
	collector.registerMetric("http_req_receiving", Trend)
	collector.registerMetric("http_req_sending", Trend)
	collector.registerMetric("http_req_tls_handshaking", Trend)
	collector.registerMetric("http_req_waiting", Trend)
	collector.registerMetric("http_reqs", Counter)

	return collector
}

// registerMetric creates and registers a new metric
func (mc *MetricsCollector) registerMetric(name string, metricType MetricType) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.metrics[name] = NewMetric(name, metricType)
}

// GetMetric returns a metric by name
func (mc *MetricsCollector) GetMetric(name string) *Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.metrics[name]
}

// GetAllMetrics returns all metrics
func (mc *MetricsCollector) GetAllMetrics() map[string]*Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	result := make(map[string]*Metric)
	for k, v := range mc.metrics {
		result[k] = v
	}
	return result
}

// RecordValue records a value for the specified metric
func (mc *MetricsCollector) RecordValue(metricName string, value float64, tags map[string]string) {
	if metric := mc.GetMetric(metricName); metric != nil {
		metric.AddValue(value, tags)
	}
}

// RecordCheck records a check result
func (mc *MetricsCollector) RecordCheck(success bool, tags map[string]string) {
	value := 0.0
	if success {
		value = 1.0
	}
	mc.RecordValue("checks", value, tags)
}

// RecordHTTPRequest records HTTP request metrics
func (mc *MetricsCollector) RecordHTTPRequest(
	duration time.Duration,
	blocked, connecting, sending, waiting, receiving, tlsHandshaking time.Duration,
	failed bool,
	dataSent, dataReceived int64,
	tags map[string]string,
) {
	mc.RecordValue("http_reqs", 1, tags)
	mc.RecordValue("http_req_duration", float64(duration.Milliseconds()), tags)
	mc.RecordValue("http_req_blocked", float64(blocked.Milliseconds()), tags)
	mc.RecordValue("http_req_connecting", float64(connecting.Milliseconds()), tags)
	mc.RecordValue("http_req_sending", float64(sending.Milliseconds()), tags)
	mc.RecordValue("http_req_waiting", float64(waiting.Milliseconds()), tags)
	mc.RecordValue("http_req_receiving", float64(receiving.Milliseconds()), tags)
	mc.RecordValue("http_req_tls_handshaking", float64(tlsHandshaking.Milliseconds()), tags)

	if failed {
		mc.RecordValue("http_req_failed", 1, tags)
	} else {
		mc.RecordValue("http_req_failed", 0, tags)
	}

	mc.RecordValue("data_sent", float64(dataSent), tags)
	mc.RecordValue("data_received", float64(dataReceived), tags)
}

// RecordIteration records iteration metrics
func (mc *MetricsCollector) RecordIteration(duration time.Duration, tags map[string]string) {
	mc.RecordValue("iterations", 1, tags)
	mc.RecordValue("iteration_duration", float64(duration.Milliseconds()), tags)
}

// UpdateVirtualUsers updates the current virtual user count
func (mc *MetricsCollector) UpdateVirtualUsers(current, max int) {
	mc.RecordValue("vus", float64(current), nil)
	mc.RecordValue("vus_max", float64(max), nil)
}

// MetricSummary provides aggregated statistics for a metric
type MetricSummary struct {
	Name        string     `json:"name"`
	Type        MetricType `json:"type"`
	Count       int        `json:"count"`
	Sum         float64    `json:"sum,omitempty"`
	Average     float64    `json:"average,omitempty"`
	Min         float64    `json:"min,omitempty"`
	Max         float64    `json:"max,omitempty"`
	P50         float64    `json:"p50,omitempty"`
	P90         float64    `json:"p90,omitempty"`
	P95         float64    `json:"p95,omitempty"`
	P99         float64    `json:"p99,omitempty"`
	LatestValue float64    `json:"latest_value,omitempty"`
	Rate        float64    `json:"rate,omitempty"`      // Rate per second
	RateUnit    string     `json:"rate_unit,omitempty"` // Unit for the rate (req/s, bytes/s, etc.)
}

// GetSummary returns a summary of the metric
func (m *Metric) GetSummary() MetricSummary {
	summary := MetricSummary{
		Name:  m.Name,
		Type:  m.Type,
		Count: m.GetCount(),
	}

	if summary.Count > 0 {
		latest := m.GetLatest()
		if latest != nil {
			summary.LatestValue = latest.Value
		}

		switch m.Type {
		case Counter:
			summary.Sum = m.GetSum()
		case Gauge:
			summary.LatestValue = latest.Value
			summary.Average = latest.Value
		case Rate:
			summary.Sum = m.GetSum()
			summary.Average = m.GetAverage()
		case Trend:
			summary.Sum = m.GetSum()
			summary.Average = m.GetAverage()
			summary.Min = m.GetMin()
			summary.Max = m.GetMax()
			summary.P50 = m.GetPercentile(50)
			summary.P90 = m.GetPercentile(90)
			summary.P95 = m.GetPercentile(95)
			summary.P99 = m.GetPercentile(99)
		}
	}

	return summary
}

// SetEndTime marks the end of the test run for rate calculations
func (mc *MetricsCollector) SetEndTime() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.endTime = time.Now()
}

// GetRuntimeSeconds returns the total runtime in seconds
func (mc *MetricsCollector) GetRuntimeSeconds() float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	endTime := mc.endTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	return endTime.Sub(mc.startTime).Seconds()
}

// GetSummary returns a summary of the metric with rate calculations
func (m *Metric) GetSummaryWithRate(runtimeSeconds float64) MetricSummary {
	summary := MetricSummary{
		Name:  m.Name,
		Type:  m.Type,
		Count: m.GetCount(),
	}

	if summary.Count > 0 {
		latest := m.GetLatest()
		if latest != nil {
			summary.LatestValue = latest.Value
		}

		switch m.Type {
		case Counter:
			summary.Sum = m.GetSum()
			if runtimeSeconds > 0 {
				summary.Rate = summary.Sum / runtimeSeconds
				switch m.Name {
				case "http_reqs", "iterations":
					summary.RateUnit = "req/s"
				case "data_sent", "data_received":
					summary.RateUnit = "bytes/s"
				default:
					summary.RateUnit = "/s"
				}
			}
		case Gauge:
			summary.LatestValue = latest.Value
			summary.Average = latest.Value
		case Rate:
			summary.Sum = m.GetSum()
			summary.Average = m.GetAverage()
		case Trend:
			summary.Sum = m.GetSum()
			summary.Average = m.GetAverage()
			summary.Min = m.GetMin()
			summary.Max = m.GetMax()
			summary.P50 = m.GetPercentile(50)
			summary.P90 = m.GetPercentile(90)
			summary.P95 = m.GetPercentile(95)
			summary.P99 = m.GetPercentile(99)
		}
	}

	return summary
}

// GetSummaries returns summaries for all metrics with rate calculations
func (mc *MetricsCollector) GetSummaries() map[string]MetricSummary {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	runtimeSeconds := mc.GetRuntimeSecondsUnsafe()
	summaries := make(map[string]MetricSummary)
	for name, metric := range mc.metrics {
		summaries[name] = metric.GetSummaryWithRate(runtimeSeconds)
	}
	return summaries
}

// GetRuntimeSecondsUnsafe returns runtime without locking (for internal use)
func (mc *MetricsCollector) GetRuntimeSecondsUnsafe() float64 {
	endTime := mc.endTime
	if endTime.IsZero() {
		endTime = time.Now()
	}
	return endTime.Sub(mc.startTime).Seconds()
}

// GetPerVUMetrics calculates metrics per virtual user
func (mc *MetricsCollector) GetPerVUMetrics(totalVUs int) map[string]float64 {
	if totalVUs == 0 {
		return make(map[string]float64)
	}

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	perVUMetrics := make(map[string]float64)
	for name, metric := range mc.metrics {
		if metric.Type == Counter {
			sum := metric.GetSum()
			perVUMetrics[name+"_per_vu"] = sum / float64(totalVUs)
		}
	}
	return perVUMetrics
}

// GetPerIterationMetrics calculates metrics per iteration
func (mc *MetricsCollector) GetPerIterationMetrics() map[string]float64 {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	perIterMetrics := make(map[string]float64)

	// Get total iterations
	iterationsMetric := mc.metrics["iterations"]
	if iterationsMetric == nil {
		return perIterMetrics
	}

	totalIterations := iterationsMetric.GetSum()
	if totalIterations == 0 {
		return perIterMetrics
	}

	for name, metric := range mc.metrics {
		if metric.Type == Counter && name != "iterations" {
			sum := metric.GetSum()
			perIterMetrics[name+"_per_iter"] = sum / totalIterations
		}
	}
	return perIterMetrics
}
