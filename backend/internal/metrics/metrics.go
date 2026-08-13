// Package metrics collects lightweight runtime counters and renders them in
// Prometheus text exposition format for scraping (e.g. by Grafana or a VPS
// monitoring agent). It is intentionally dependency-free and lock-free: all
// counters are atomics, and label-keyed maps are guarded by a single mutex.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Metrics holds the counters exposed via /api/metrics.
type Metrics struct {
	// httpRequests maps "METHOD|STATUS" -> count.
	httpRequestsMu sync.Mutex
	httpRequests   map[string]*atomic.Int64

	httpErrors atomic.Int64
	reqSumMs   atomic.Int64
	reqCount   atomic.Int64

	fetchCycles       atomic.Int64
	fetchCyclesFailed atomic.Int64

	aiCalls       atomic.Int64
	aiCallsFailed atomic.Int64
}

// New returns an empty Metrics collector.
func New() *Metrics {
	return &Metrics{httpRequests: make(map[string]*atomic.Int64)}
}

// HTTPRequest records one finished request with its status code.
func (m *Metrics) HTTPRequest(method string, status int) {
	if m == nil {
		return
	}
	key := method + "|" + fmt.Sprintf("%d", status)
	m.httpRequestsMu.Lock()
	c, ok := m.httpRequests[key]
	if !ok {
		c = &atomic.Int64{}
		m.httpRequests[key] = c
	}
	m.httpRequestsMu.Unlock()
	c.Add(1)
	if status >= 400 {
		m.httpErrors.Add(1)
	}
}

// RequestDuration records a request latency in milliseconds.
func (m *Metrics) RequestDuration(ms int64) {
	if m == nil {
		return
	}
	m.reqSumMs.Add(ms)
	m.reqCount.Add(1)
}

// FetchCycle records one completed fetch cycle; failed reports whether the
// cycle ended with an error.
func (m *Metrics) FetchCycle(failed bool) {
	if m == nil {
		return
	}
	m.fetchCycles.Add(1)
	if failed {
		m.fetchCyclesFailed.Add(1)
	}
}

// AICall records one upstream AI request (chat or image generation); failed
// reports whether it ended in an error.
func (m *Metrics) AICall(failed bool) {
	if m == nil {
		return
	}
	m.aiCalls.Add(1)
	if failed {
		m.aiCallsFailed.Add(1)
	}
}

// WritePrometheus renders all counters in Prometheus text exposition format.
func (m *Metrics) WritePrometheus(w io.Writer) {
	if m == nil {
		return
	}

	// HTTP requests by method/status.
	m.httpRequestsMu.Lock()
	keys := make([]string, 0, len(m.httpRequests))
	for k := range m.httpRequests {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]struct {
		key string
		val *atomic.Int64
	}, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, struct {
			key string
			val *atomic.Int64
		}{k, m.httpRequests[k]})
	}
	m.httpRequestsMu.Unlock()

	fmt.Fprintln(w, "# HELP neuralwire_http_requests_total Total HTTP requests by method and status code.")
	fmt.Fprintln(w, "# TYPE neuralwire_http_requests_total counter")
	for _, p := range pairs {
		parts := strings.SplitN(p.key, "|", 2)
		fmt.Fprintf(w, "neuralwire_http_requests_total{method=%q,status=%q} %d\n",
			parts[0], parts[1], p.val.Load())
	}

	fmt.Fprintln(w, "# HELP neuralwire_http_errors_total Total HTTP responses with status >= 400.")
	fmt.Fprintln(w, "# TYPE neuralwire_http_errors_total counter")
	fmt.Fprintf(w, "neuralwire_http_errors_total %d\n", m.httpErrors.Load())

	fmt.Fprintln(w, "# HELP neuralwire_http_request_duration_seconds HTTP request latency.")
	fmt.Fprintln(w, "# TYPE neuralwire_http_request_duration_seconds summary")
	fmt.Fprintf(w, "neuralwire_http_request_duration_seconds_sum %d\n", m.reqSumMs.Load())
	fmt.Fprintf(w, "neuralwire_http_request_duration_seconds_count %d\n", m.reqCount.Load())

	fmt.Fprintln(w, "# HELP neuralwire_fetch_cycles_total RSS fetch cycles completed.")
	fmt.Fprintln(w, "# TYPE neuralwire_fetch_cycles_total counter")
	fmt.Fprintf(w, "neuralwire_fetch_cycles_total %d\n", m.fetchCycles.Load())

	fmt.Fprintln(w, "# HELP neuralwire_fetch_cycles_failed_total RSS fetch cycles that ended with an error.")
	fmt.Fprintln(w, "# TYPE neuralwire_fetch_cycles_failed_total counter")
	fmt.Fprintf(w, "neuralwire_fetch_cycles_failed_total %d\n", m.fetchCyclesFailed.Load())

	fmt.Fprintln(w, "# HELP neuralwire_ai_calls_total Upstream AI requests (summarize/categorize/score/image).")
	fmt.Fprintln(w, "# TYPE neuralwire_ai_calls_total counter")
	fmt.Fprintf(w, "neuralwire_ai_calls_total %d\n", m.aiCalls.Load())

	fmt.Fprintln(w, "# HELP neuralwire_ai_calls_failed_total Upstream AI requests that failed.")
	fmt.Fprintln(w, "# TYPE neuralwire_ai_calls_failed_total counter")
	fmt.Fprintf(w, "neuralwire_ai_calls_failed_total %d\n", m.aiCallsFailed.Load())
}
