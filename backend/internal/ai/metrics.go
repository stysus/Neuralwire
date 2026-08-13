package ai

import "neuralwire/backend/internal/metrics"

// defaultMetrics is the shared collector for upstream AI request counters. It
// is replaced by the application via SetMetrics so /api/metrics can report AI
// call volume; tests leave it as an empty collector, which is a safe no-op.
var defaultMetrics = metrics.New()

// SetMetrics installs the metrics collector used to count upstream AI calls.
// A nil argument restores the empty default collector.
func SetMetrics(m *metrics.Metrics) {
	if m == nil {
		defaultMetrics = metrics.New()
		return
	}
	defaultMetrics = m
}

// recordAICall reports one upstream AI request to the metrics collector.
func recordAICall(failed bool) {
	defaultMetrics.AICall(failed)
}
