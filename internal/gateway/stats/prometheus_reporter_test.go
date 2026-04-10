package stats

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
	tallyprom "github.com/uber-go/tally/v4/prometheus"
)

// Global test reporter to avoid route conflicts.
var (
	testReporter tallyprom.Reporter
	testOnce     sync.Once
	testErr      error
)

func getTestReporter(t *testing.T) tallyprom.Reporter {
	testOnce.Do(func() {
		testReporter, testErr = NewPrometheusReporter()
	})
	require.NoError(t, testErr)
	require.NotNil(t, testReporter)
	return testReporter
}

func TestNewPrometheusReporter(t *testing.T) {
	t.Run("creates prometheus reporter successfully", func(t *testing.T) {
		reporter := getTestReporter(t)

		assert.NotNil(t, reporter, "Reporter should not be nil")
		// Check that it implements the Reporter interface
		_ = tallyprom.Reporter(reporter)
	})
}

func TestPrometheusReporter_Interfaces(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("implements CachedStatsReporter", func(t *testing.T) {
		assert.Implements(t, (*tally.CachedStatsReporter)(nil), reporter, "Should implement tally.CachedStatsReporter interface")
	})

	t.Run("implements BaseStatsReporter", func(t *testing.T) {
		assert.Implements(t, (*tally.BaseStatsReporter)(nil), reporter, "Should implement tally.BaseStatsReporter interface")
	})

	t.Run("implements Capabilities", func(t *testing.T) {
		assert.Implements(t, (*tally.Capabilities)(nil), reporter, "Should implement tally.Capabilities interface")
	})
}

func TestPrometheusReporter_HTTPHandler(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("provides HTTP handler", func(t *testing.T) {
		handler := reporter.HTTPHandler()

		assert.NotNil(t, handler, "HTTPHandler should not return nil")
		assert.Implements(t, (*http.Handler)(nil), handler, "Should implement http.Handler interface")
	})

	t.Run("handler consistency", func(t *testing.T) {
		handler1 := reporter.HTTPHandler()
		handler2 := reporter.HTTPHandler()

		assert.NotNil(t, handler1, "First handler should not be nil")
		assert.NotNil(t, handler2, "Second handler should not be nil")

		// Test that both handlers behave the same way instead of direct comparison
		req1 := httptest.NewRequest("GET", "/metrics", nil)
		w1 := httptest.NewRecorder()
		handler1.ServeHTTP(w1, req1)

		req2 := httptest.NewRequest("GET", "/metrics", nil)
		w2 := httptest.NewRecorder()
		handler2.ServeHTTP(w2, req2)

		assert.Equal(t, w1.Code, w2.Code, "Both handlers should return same status code")
		assert.Equal(t, w1.Header().Get("Content-Type"), w2.Header().Get("Content-Type"), "Both handlers should return same content type")
	})
}

func TestPrometheusReporter_MetricsEndpoint(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("serves metrics over HTTP", func(t *testing.T) {
		// Create a test HTTP server with the Prometheus handler
		handler := reporter.HTTPHandler()
		server := httptest.NewServer(handler)
		defer server.Close()

		// Make a request to the metrics endpoint
		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		// Verify response
		assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")
		contentType := resp.Header.Get("Content-Type")
		assert.Contains(t, contentType, "text/plain", "Should have text/plain content type")
	})

	t.Run("metrics endpoint contains expected headers", func(t *testing.T) {
		handler := reporter.HTTPHandler()
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	})
}

func TestPrometheusReporter_Capabilities(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("reporting capability", func(t *testing.T) {
		capabilities := reporter.Capabilities()
		assert.NotNil(t, capabilities, "Capabilities should not be nil")
		assert.True(t, capabilities.Reporting(), "Should support reporting")
	})

	t.Run("tagging capability", func(t *testing.T) {
		capabilities := reporter.Capabilities()
		assert.NotNil(t, capabilities, "Capabilities should not be nil")
		assert.True(t, capabilities.Tagging(), "Should support tagging")
	})
}

func TestPrometheusReporter_FlushOperations(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("flush does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			reporter.Flush()
		}, "Flush should not panic")
	})

	t.Run("multiple flushes don't cause issues", func(t *testing.T) {
		assert.NotPanics(t, func() {
			for i := 0; i < 5; i++ {
				reporter.Flush()
			}
		}, "Multiple flushes should not panic")
	})
}

func TestPrometheusReporter_RegisterMethods(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("register counter", func(t *testing.T) {
		counter, err := reporter.RegisterCounter("test_counter_unique", []string{"tag1", "tag2"}, "Test counter description")
		assert.NoError(t, err, "RegisterCounter should not return error")
		assert.NotNil(t, counter, "Counter should not be nil")
	})

	t.Run("register gauge", func(t *testing.T) {
		gauge, err := reporter.RegisterGauge("test_gauge_unique", []string{"tag1", "tag2"}, "Test gauge description")
		assert.NoError(t, err, "RegisterGauge should not return error")
		assert.NotNil(t, gauge, "Gauge should not be nil")
	})

	t.Run("register timer", func(t *testing.T) {
		timer, err := reporter.RegisterTimer("test_timer_unique", []string{"tag1", "tag2"}, "Test timer description", nil)
		assert.NoError(t, err, "RegisterTimer should not return error")
		assert.NotNil(t, timer, "Timer should not be nil")
	})
}

func TestPrometheusReporter_EdgeCases(t *testing.T) {
	reporter := getTestReporter(t)

	t.Run("register with nil tag keys", func(t *testing.T) {
		counter, err := reporter.RegisterCounter("nil_tags_counter_unique", nil, "Nil tags counter")
		assert.NoError(t, err, "Should handle nil tag keys")
		assert.NotNil(t, counter, "Counter should not be nil")
	})

	t.Run("register with empty description", func(t *testing.T) {
		counter, err := reporter.RegisterCounter("empty_desc_counter_unique", []string{"tag"}, "")
		assert.NoError(t, err, "Should handle empty description")
		assert.NotNil(t, counter, "Counter should not be nil")
	})
}

func TestPrometheusReporter_HTTPHandlerContent(t *testing.T) {
	reporter := getTestReporter(t)

	// Register some metrics to ensure they appear in output
	_, err := reporter.RegisterCounter("http_requests_total_unique", []string{"method", "endpoint"}, "Total HTTP requests")
	require.NoError(t, err)

	_, err = reporter.RegisterGauge("memory_usage_bytes_unique", []string{"type"}, "Memory usage in bytes")
	require.NoError(t, err)

	t.Run("metrics output contains prometheus format", func(t *testing.T) {
		handler := reporter.HTTPHandler()
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		responseBody := w.Body.String()

		// Check for basic Prometheus format elements
		assert.Contains(t, responseBody, "# HELP", "Should contain HELP comments")
		assert.Contains(t, responseBody, "# TYPE", "Should contain TYPE comments")
	})
}

// Benchmark tests for performance measurement.
func BenchmarkNewPrometheusReporter(b *testing.B) {
	// Skip benchmark to avoid route conflicts
	b.Skip("Skipping to avoid /metrics route conflicts")
}

func BenchmarkPrometheusReporter_HTTPHandler(b *testing.B) {
	// Use shared test reporter for benchmarks
	testOnce.Do(func() {
		testReporter, testErr = NewPrometheusReporter()
	})
	if testErr != nil {
		b.Fatal(testErr)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler := testReporter.HTTPHandler()
		_ = handler
	}
}

func BenchmarkPrometheusReporter_RegisterCounter(b *testing.B) {
	// Use shared test reporter for benchmarks
	testOnce.Do(func() {
		testReporter, testErr = NewPrometheusReporter()
	})
	if testErr != nil {
		b.Fatal(testErr)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique names to avoid conflicts
		name := "benchmark_counter_" + string(rune(i%1000))
		_, err := testReporter.RegisterCounter(name, []string{"tag"}, "Benchmark counter")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPrometheusReporter_HTTPServe(b *testing.B) {
	// Use shared test reporter for benchmarks
	testOnce.Do(func() {
		testReporter, testErr = NewPrometheusReporter()
	})
	if testErr != nil {
		b.Fatal(testErr)
	}

	// Register some metrics first
	_, err := testReporter.RegisterCounter("bench_counter_serve", []string{"type"}, "Benchmark counter")
	if err != nil {
		b.Fatal(err)
	}

	handler := testReporter.HTTPHandler()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "/metrics", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}
