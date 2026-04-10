package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Test helper to create a logger and capture output.
func createTestLogger() (*zap.Logger, *bytes.Buffer, *bufio.Writer) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	logger := zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(w),
			zap.DebugLevel,
		),
	)
	return logger, &b, w
}

// Test helper to flush logger and get output.
func flushAndGetOutput(t *testing.T, logger *zap.Logger, w *bufio.Writer, b *bytes.Buffer) string {
	require.NoError(t, logger.Sync())
	require.NoError(t, w.Flush())
	return b.String()
}

// Test helper to parse JSON log output.
func parseLogOutput(t *testing.T, output string) map[string]interface{} {
	var logData map[string]interface{}
	err := json.Unmarshal([]byte(output), &logData)
	require.NoError(t, err, "Output should be valid JSON")
	return logData
}

func TestNewLogReporter(t *testing.T) {
	t.Run("creates log reporter with valid logger", func(t *testing.T) {
		logger, _, _ := createTestLogger()
		reporter := NewLogReporter(logger)

		assert.NotNil(t, reporter)
		assert.IsType(t, &logReporter{}, reporter)

		// Verify it implements tally.StatsReporter interface
		var _ = reporter
	})

	t.Run("creates log reporter with nil logger", func(t *testing.T) {
		reporter := NewLogReporter(nil)

		assert.NotNil(t, reporter)
		assert.IsType(t, &logReporter{}, reporter)
	})
}

func TestLogReporter_Capabilities(t *testing.T) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger).(*logReporter)

	t.Run("reporting capability", func(t *testing.T) {
		assert.True(t, reporter.Reporting())
	})

	t.Run("tagging capability", func(t *testing.T) {
		assert.True(t, reporter.Tagging())
	})

	t.Run("capabilities interface", func(t *testing.T) {
		capabilities := reporter.Capabilities()
		assert.NotNil(t, capabilities)
		assert.Equal(t, reporter, capabilities)
		assert.True(t, capabilities.Reporting())
		assert.True(t, capabilities.Tagging())
	})
}

func TestLogReporter_Flush(t *testing.T) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger).(*logReporter)

	t.Run("flush does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			reporter.Flush()
		})
	})
}

func TestLogReporter_ReportCounter(t *testing.T) {
	testCases := []struct {
		name            string
		metricName      string
		tags            map[string]string
		value           int64
		expectedContent []string
		description     string
	}{
		{
			name:            "counter with basic values",
			metricName:      "test_counter",
			tags:            map[string]string{"service": "test"},
			value:           42,
			expectedContent: []string{"counter", "test_counter", "42", "service", "test", "\"type\":\"counter\""},
			description:     "Should log counter with all fields",
		},
		{
			name:            "counter with zero value",
			metricName:      "zero_counter",
			tags:            map[string]string{},
			value:           0,
			expectedContent: []string{"counter", "zero_counter", "0", "\"type\":\"counter\""},
			description:     "Should handle zero counter value",
		},
		{
			name:            "counter with negative value",
			metricName:      "negative_counter",
			tags:            map[string]string{"env": "test"},
			value:           -10,
			expectedContent: []string{"counter", "negative_counter", "-10", "env", "\"type\":\"counter\""},
			description:     "Should handle negative counter value",
		},
		{
			name:            "counter with multiple tags",
			metricName:      "multi_tag_counter",
			tags:            map[string]string{"service": "api", "version": "v1", "env": "prod"},
			value:           100,
			expectedContent: []string{"counter", "multi_tag_counter", "100", "service", "version", "env", "\"type\":\"counter\""},
			description:     "Should handle multiple tags",
		},
		{
			name:            "counter with nil tags",
			metricName:      "nil_tags_counter",
			tags:            nil,
			value:           25,
			expectedContent: []string{"counter", "nil_tags_counter", "25", "\"type\":\"counter\""},
			description:     "Should handle nil tags",
		},
		{
			name:            "counter with empty name",
			metricName:      "",
			tags:            map[string]string{"tag": "value"},
			value:           1,
			expectedContent: []string{"counter", "1", "tag", "\"type\":\"counter\""},
			description:     "Should handle empty metric name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, b, w := createTestLogger()
			reporter := NewLogReporter(logger).(*logReporter)

			reporter.ReportCounter(tc.metricName, tc.tags, tc.value)
			output := flushAndGetOutput(t, logger, w, b)

			// Verify expected content is present
			for _, content := range tc.expectedContent {
				assert.Contains(t, output, content, "Output should contain: %s", content)
			}

			// Verify valid JSON structure
			logData := parseLogOutput(t, output)
			assert.Equal(t, "counter", logData["msg"])
			assert.Equal(t, tc.metricName, logData["name"])
			assert.Equal(t, float64(tc.value), logData["value"])
			assert.Equal(t, "counter", logData["type"])

			if tc.tags != nil {
				assert.Contains(t, logData, "tags")
			}
		})
	}
}

func TestLogReporter_ReportGauge(t *testing.T) {
	testCases := []struct {
		name            string
		metricName      string
		tags            map[string]string
		value           float64
		expectedContent []string
		description     string
	}{
		{
			name:            "gauge with float value",
			metricName:      "test_gauge",
			tags:            map[string]string{"type": "memory"},
			value:           42.5,
			expectedContent: []string{"gauge", "test_gauge", "42.5", "type", "memory", "\"type\":\"gauge\""},
			description:     "Should log gauge with float value",
		},
		{
			name:            "gauge with zero value",
			metricName:      "zero_gauge",
			tags:            map[string]string{},
			value:           0.0,
			expectedContent: []string{"gauge", "zero_gauge", "0", "\"type\":\"gauge\""},
			description:     "Should handle zero gauge value",
		},
		{
			name:            "gauge with negative value",
			metricName:      "negative_gauge",
			tags:            map[string]string{"direction": "down"},
			value:           -3.14,
			expectedContent: []string{"gauge", "negative_gauge", "-3.14", "direction", "\"type\":\"gauge\""},
			description:     "Should handle negative gauge value",
		},
		{
			name:            "gauge with large value",
			metricName:      "large_gauge",
			tags:            map[string]string{"unit": "bytes"},
			value:           1.23e+10,
			expectedContent: []string{"gauge", "large_gauge", "12300000000", "unit", "\"type\":\"gauge\""},
			description:     "Should handle large gauge value",
		},
		{
			name:            "gauge with precision value",
			metricName:      "precision_gauge",
			tags:            map[string]string{"precision": "high"},
			value:           0.000001,
			expectedContent: []string{"gauge", "precision_gauge", "0.000001", "precision", "\"type\":\"gauge\""},
			description:     "Should handle high precision gauge value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, b, w := createTestLogger()
			reporter := NewLogReporter(logger).(*logReporter)

			reporter.ReportGauge(tc.metricName, tc.tags, tc.value)
			output := flushAndGetOutput(t, logger, w, b)

			// Verify expected content is present
			for _, content := range tc.expectedContent {
				assert.Contains(t, output, content, "Output should contain: %s", content)
			}

			// Verify valid JSON structure
			logData := parseLogOutput(t, output)
			assert.Equal(t, "gauge", logData["msg"])
			assert.Equal(t, tc.metricName, logData["name"])
			assert.Equal(t, tc.value, logData["value"])
			assert.Equal(t, "gauge", logData["type"])
		})
	}
}

func TestLogReporter_ReportTimer(t *testing.T) {
	testCases := []struct {
		name            string
		metricName      string
		tags            map[string]string
		interval        time.Duration
		expectedContent []string
		description     string
	}{
		{
			name:            "timer with milliseconds",
			metricName:      "request_duration",
			tags:            map[string]string{"endpoint": "/api/v1"},
			interval:        100 * time.Millisecond,
			expectedContent: []string{"timer", "request_duration", "0.1", "endpoint", "\"type\":\"timer\""},
			description:     "Should log timer with milliseconds",
		},
		{
			name:            "timer with seconds",
			metricName:      "operation_time",
			tags:            map[string]string{"operation": "backup"},
			interval:        5 * time.Second,
			expectedContent: []string{"timer", "operation_time", "5", "operation", "\"type\":\"timer\""},
			description:     "Should log timer with seconds",
		},
		{
			name:            "timer with nanoseconds",
			metricName:      "cpu_time",
			tags:            map[string]string{"cpu": "0"},
			interval:        1500 * time.Nanosecond,
			expectedContent: []string{"timer", "cpu_time", "0.0000015", "cpu", "\"type\":\"timer\""},
			description:     "Should log timer with nanoseconds",
		},
		{
			name:            "timer with zero duration",
			metricName:      "zero_timer",
			tags:            map[string]string{},
			interval:        0,
			expectedContent: []string{"timer", "zero_timer", "0", "\"type\":\"timer\""},
			description:     "Should handle zero duration",
		},
		{
			name:            "timer with large duration",
			metricName:      "long_timer",
			tags:            map[string]string{"type": "batch"},
			interval:        24 * time.Hour,
			expectedContent: []string{"timer", "long_timer", "86400", "type", "\"type\":\"timer\""},
			description:     "Should handle large duration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, b, w := createTestLogger()
			reporter := NewLogReporter(logger).(*logReporter)

			reporter.ReportTimer(tc.metricName, tc.tags, tc.interval)
			output := flushAndGetOutput(t, logger, w, b)

			// Verify expected content is present
			for _, content := range tc.expectedContent {
				assert.Contains(t, output, content, "Output should contain: %s", content)
			}

			// Verify valid JSON structure
			logData := parseLogOutput(t, output)
			assert.Equal(t, "timer", logData["msg"])
			assert.Equal(t, tc.metricName, logData["name"])
			assert.Equal(t, "timer", logData["type"])
			assert.Contains(t, logData, "value")
		})
	}
}

func TestLogReporter_ReportHistogramValueSamples(t *testing.T) {
	testCases := []struct {
		name             string
		metricName       string
		tags             map[string]string
		buckets          tally.Buckets
		bucketLowerBound float64
		bucketUpperBound float64
		samples          int64
		expectedContent  []string
		description      string
	}{
		{
			name:             "value histogram with linear buckets",
			metricName:       "response_size",
			tags:             map[string]string{"handler": "api"},
			buckets:          tally.MustMakeLinearValueBuckets(0, 100, 10),
			bucketLowerBound: 100.0,
			bucketUpperBound: 200.0,
			samples:          25,
			expectedContent:  []string{"histogram", "response_size", "handler", "api", "100", "200", "25", "\"type\":\"valueHistogram\""},
			description:      "Should log value histogram with linear buckets",
		},
		{
			name:             "value histogram with exponential buckets",
			metricName:       "request_latency",
			tags:             map[string]string{"service": "auth"},
			buckets:          tally.MustMakeExponentialValueBuckets(1, 2, 8),
			bucketLowerBound: 8.0,
			bucketUpperBound: 16.0,
			samples:          42,
			expectedContent:  []string{"histogram", "request_latency", "service", "auth", "8", "16", "42", "\"type\":\"valueHistogram\""},
			description:      "Should log value histogram with exponential buckets",
		},
		{
			name:             "value histogram with zero samples",
			metricName:       "zero_samples",
			tags:             map[string]string{},
			buckets:          tally.MustMakeLinearValueBuckets(0, 10, 5),
			bucketLowerBound: 10.0,
			bucketUpperBound: 20.0,
			samples:          0,
			expectedContent:  []string{"histogram", "zero_samples", "10", "20", "0", "\"type\":\"valueHistogram\""},
			description:      "Should handle zero samples",
		},
		{
			name:             "value histogram with negative bounds",
			metricName:       "negative_bounds",
			tags:             map[string]string{"type": "delta"},
			buckets:          tally.MustMakeLinearValueBuckets(-100, 20, 10),
			bucketLowerBound: -50.0,
			bucketUpperBound: -30.0,
			samples:          15,
			expectedContent:  []string{"histogram", "negative_bounds", "type", "delta", "-50", "-30", "15", "\"type\":\"valueHistogram\""},
			description:      "Should handle negative bounds",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, b, w := createTestLogger()
			reporter := NewLogReporter(logger).(*logReporter)

			reporter.ReportHistogramValueSamples(
				tc.metricName,
				tc.tags,
				tc.buckets,
				tc.bucketLowerBound,
				tc.bucketUpperBound,
				tc.samples,
			)
			output := flushAndGetOutput(t, logger, w, b)

			// Verify expected content is present
			for _, content := range tc.expectedContent {
				assert.Contains(t, output, content, "Output should contain: %s", content)
			}

			// Verify valid JSON structure
			logData := parseLogOutput(t, output)
			assert.Equal(t, "histogram", logData["msg"])
			assert.Equal(t, tc.metricName, logData["name"])
			assert.Equal(t, "valueHistogram", logData["type"])
			assert.Contains(t, logData, "buckets")
			assert.Equal(t, tc.bucketLowerBound, logData["bucketLowerBound"])
			assert.Equal(t, tc.bucketUpperBound, logData["bucketUpperBound"])
			assert.Equal(t, float64(tc.samples), logData["samples"])
		})
	}
}

func TestLogReporter_ReportHistogramDurationSamples(t *testing.T) {
	testCases := []struct {
		name             string
		metricName       string
		tags             map[string]string
		buckets          tally.Buckets
		bucketLowerBound time.Duration
		bucketUpperBound time.Duration
		samples          int64
		expectedContent  []string
		description      string
	}{
		{
			name:             "duration histogram with milliseconds",
			metricName:       "http_request_duration",
			tags:             map[string]string{"method": "GET"},
			buckets:          tally.MustMakeExponentialDurationBuckets(time.Millisecond, 2, 10),
			bucketLowerBound: 100 * time.Millisecond,
			bucketUpperBound: 200 * time.Millisecond,
			samples:          50,
			expectedContent:  []string{"histogram", "http_request_duration", "method", "GET", "0.1", "0.2", "50", "\"type\":\"durationHistogram\""},
			description:      "Should log duration histogram with milliseconds",
		},
		{
			name:             "duration histogram with seconds",
			metricName:       "db_query_duration",
			tags:             map[string]string{"query": "select"},
			buckets:          tally.MustMakeLinearDurationBuckets(0, time.Second, 5),
			bucketLowerBound: 2 * time.Second,
			bucketUpperBound: 3 * time.Second,
			samples:          10,
			expectedContent:  []string{"histogram", "db_query_duration", "query", "select", "2", "3", "10", "\"type\":\"durationHistogram\""},
			description:      "Should log duration histogram with seconds",
		},
		{
			name:             "duration histogram with microseconds",
			metricName:       "cpu_instruction_time",
			tags:             map[string]string{"core": "0"},
			buckets:          tally.MustMakeExponentialDurationBuckets(time.Microsecond, 2, 8),
			bucketLowerBound: 50 * time.Microsecond,
			bucketUpperBound: 100 * time.Microsecond,
			samples:          1000,
			expectedContent:  []string{"histogram", "cpu_instruction_time", "core", "0", "0.00005", "0.0001", "1000", "\"type\":\"durationHistogram\""},
			description:      "Should log duration histogram with microseconds",
		},
		{
			name:             "duration histogram with zero duration",
			metricName:       "zero_duration_hist",
			tags:             map[string]string{},
			buckets:          tally.MustMakeLinearDurationBuckets(0, time.Millisecond, 3),
			bucketLowerBound: 0,
			bucketUpperBound: 0,
			samples:          5,
			expectedContent:  []string{"histogram", "zero_duration_hist", "0", "0", "5", "\"type\":\"durationHistogram\""},
			description:      "Should handle zero duration bounds",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, b, w := createTestLogger()
			reporter := NewLogReporter(logger).(*logReporter)

			reporter.ReportHistogramDurationSamples(
				tc.metricName,
				tc.tags,
				tc.buckets,
				tc.bucketLowerBound,
				tc.bucketUpperBound,
				tc.samples,
			)
			output := flushAndGetOutput(t, logger, w, b)

			// Verify expected content is present
			for _, content := range tc.expectedContent {
				assert.Contains(t, output, content, "Output should contain: %s", content)
			}

			// Verify valid JSON structure
			logData := parseLogOutput(t, output)
			assert.Equal(t, "histogram", logData["msg"])
			assert.Equal(t, tc.metricName, logData["name"])
			assert.Equal(t, "durationHistogram", logData["type"])
			assert.Contains(t, logData, "buckets")
			assert.Equal(t, float64(tc.samples), logData["samples"])
		})
	}
}

func TestLogReporter_InterfaceCompliance(t *testing.T) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger)

	t.Run("implements tally.StatsReporter", func(t *testing.T) {
		var _ = reporter
	})

	t.Run("implements tally.Capabilities", func(t *testing.T) {
		capabilities := reporter.Capabilities()
		var _ = capabilities
	})
}

func TestLogReporter_EdgeCases(t *testing.T) {
	t.Run("nil logger handling", func(t *testing.T) {
		reporter := NewLogReporter(nil).(*logReporter)

		// These should panic with nil logger since Debug() is called on nil
		assert.Panics(t, func() {
			reporter.ReportCounter("test", map[string]string{"tag": "value"}, 1)
		})

		assert.Panics(t, func() {
			reporter.ReportGauge("test", map[string]string{"tag": "value"}, 1.0)
		})

		assert.Panics(t, func() {
			reporter.ReportTimer("test", map[string]string{"tag": "value"}, time.Second)
		})
	})

	t.Run("empty metric names", func(t *testing.T) {
		logger, b, w := createTestLogger()
		reporter := NewLogReporter(logger).(*logReporter)

		reporter.ReportCounter("", nil, 1)
		output := flushAndGetOutput(t, logger, w, b)

		logData := parseLogOutput(t, output)
		assert.Equal(t, "", logData["name"])
		assert.Equal(t, "counter", logData["type"])
	})

	t.Run("special characters in metric names and tags", func(t *testing.T) {
		logger, b, w := createTestLogger()
		reporter := NewLogReporter(logger).(*logReporter)

		specialName := "metric.with-special_chars:123"
		specialTags := map[string]string{
			"key-with.special:chars": "value_with-special.chars",
			"unicode-key":            "unicode-value-🚀",
		}

		reporter.ReportCounter(specialName, specialTags, 42)
		output := flushAndGetOutput(t, logger, w, b)

		logData := parseLogOutput(t, output)
		assert.Equal(t, specialName, logData["name"])
		assert.Contains(t, logData, "tags")
	})
}

// Benchmark tests for performance measurement.
func BenchmarkLogReporter_ReportCounter(b *testing.B) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger).(*logReporter)
	tags := map[string]string{"service": "test", "env": "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reporter.ReportCounter("benchmark_counter", tags, int64(i))
	}
}

func BenchmarkLogReporter_ReportGauge(b *testing.B) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger).(*logReporter)
	tags := map[string]string{"service": "test", "env": "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reporter.ReportGauge("benchmark_gauge", tags, float64(i))
	}
}

func BenchmarkLogReporter_ReportTimer(b *testing.B) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger).(*logReporter)
	tags := map[string]string{"service": "test", "env": "benchmark"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reporter.ReportTimer("benchmark_timer", tags, time.Duration(i)*time.Millisecond)
	}
}

func BenchmarkLogReporter_ReportHistogram(b *testing.B) {
	logger, _, _ := createTestLogger()
	reporter := NewLogReporter(logger).(*logReporter)
	tags := map[string]string{"service": "test", "env": "benchmark"}
	buckets := tally.MustMakeLinearValueBuckets(0, 100, 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reporter.ReportHistogramValueSamples("benchmark_histogram", tags, buckets, 100.0, 200.0, int64(i))
	}
}
