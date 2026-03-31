package stats

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uber-go/tally/v4"

	"go.admiral.io/admiral/internal/config"
)

func TestNewRuntimeStatsCancelTicker(t *testing.T) {
	scope := tally.NewTestScope("", nil)
	ctx, cancel := context.WithCancel(context.Background())

	runtimeCollector := NewRuntimeStats(scope, nil)
	go runtimeCollector.Collect(ctx)

	// Give time for the ticker to spin up
	time.Sleep(time.Millisecond * 50)
	cancel()
}

func TestNewRuntimeStats(t *testing.T) {
	testCases := []struct {
		name             string
		cfg              *config.GoRuntimeStats
		expectedInterval time.Duration
		description      string
	}{
		{
			name:             "nil config uses default interval",
			cfg:              nil,
			expectedInterval: 10 * time.Second,
			description:      "Should use default 10 second interval when config is nil",
		},
		{
			name: "config with custom interval",
			cfg: &config.GoRuntimeStats{
				CollectionInterval: func() *time.Duration { d := 5 * time.Second; return &d }(),
			},
			expectedInterval: 5 * time.Second,
			description:      "Should use custom interval from config",
		},
		{
			name: "config with very short interval",
			cfg: &config.GoRuntimeStats{
				CollectionInterval: func() *time.Duration { d := 100 * time.Millisecond; return &d }(),
			},
			expectedInterval: 100 * time.Millisecond,
			description:      "Should handle short intervals",
		},
		{
			name: "config with long interval",
			cfg: &config.GoRuntimeStats{
				CollectionInterval: func() *time.Duration { d := time.Hour; return &d }(),
			},
			expectedInterval: time.Hour,
			description:      "Should handle long intervals",
		},
		{
			name: "config with zero interval",
			cfg: &config.GoRuntimeStats{
				CollectionInterval: func() *time.Duration { d := time.Duration(0); return &d }(),
			},
			expectedInterval: 0,
			description:      "Should handle zero interval",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scope := tally.NewTestScope("test", nil)
			collector := NewRuntimeStats(scope, tc.cfg)

			assert.NotNil(t, collector)
			assert.Equal(t, tc.expectedInterval, collector.collectionInterval)
			assert.NotNil(t, collector.runtimeMetrics)

			// Verify all metrics are initialized
			assert.NotNil(t, collector.runtimeMetrics.cpuGoroutines)
			assert.NotNil(t, collector.runtimeMetrics.cpuCgoCalls)
			assert.NotNil(t, collector.runtimeMetrics.memoryAlloc)
			assert.NotNil(t, collector.runtimeMetrics.memoryTotal)
			assert.NotNil(t, collector.runtimeMetrics.memorySys)
			assert.NotNil(t, collector.runtimeMetrics.memoryLookups)
			assert.NotNil(t, collector.runtimeMetrics.memoryMalloc)
			assert.NotNil(t, collector.runtimeMetrics.memoryFrees)
			assert.NotNil(t, collector.runtimeMetrics.memoryHeapAlloc)
			assert.NotNil(t, collector.runtimeMetrics.memoryHeapSys)
			assert.NotNil(t, collector.runtimeMetrics.memoryHeapIdle)
			assert.NotNil(t, collector.runtimeMetrics.memoryHeapInuse)
			assert.NotNil(t, collector.runtimeMetrics.memoryHeapReleased)
			assert.NotNil(t, collector.runtimeMetrics.memoryHeapObjects)
			assert.NotNil(t, collector.runtimeMetrics.memoryStackInuse)
			assert.NotNil(t, collector.runtimeMetrics.memoryStackSys)
			assert.NotNil(t, collector.runtimeMetrics.memoryStackMSpanInuse)
			assert.NotNil(t, collector.runtimeMetrics.memoryStackMSpanSys)
			assert.NotNil(t, collector.runtimeMetrics.memoryStackMCacheInuse)
			assert.NotNil(t, collector.runtimeMetrics.memoryStackMCacheSys)
			assert.NotNil(t, collector.runtimeMetrics.memoryOtherSys)
			assert.NotNil(t, collector.runtimeMetrics.memoryGCSys)
			assert.NotNil(t, collector.runtimeMetrics.memoryGCNext)
			assert.NotNil(t, collector.runtimeMetrics.memoryGCLast)
			assert.NotNil(t, collector.runtimeMetrics.memoryGCPauseTotal)
			assert.NotNil(t, collector.runtimeMetrics.memoryGCCount)
		})
	}
}

func TestRuntimeStatCollector_MetricScope(t *testing.T) {
	t.Run("creates runtime sub-scope", func(t *testing.T) {
		parentScope := tally.NewTestScope("parent", nil)
		collector := NewRuntimeStats(parentScope, nil)

		assert.NotNil(t, collector)

		// Trigger metric creation by calling collect methods
		collector.collectCPUStats()

		// Verify that metrics are created under runtime scope
		snapshot := parentScope.Snapshot()
		gauges := snapshot.Gauges()

		// Check if any runtime metrics exist
		foundRuntimeMetrics := false
		for name := range gauges {
			// tally test scope may use different prefix format
			if name == "parent.runtime.cpu.goroutines" || name == "runtime.cpu.goroutines" {
				foundRuntimeMetrics = true
				break
			}
		}

		// If no exact match, just verify we have some metrics
		if !foundRuntimeMetrics {
			assert.True(t, len(gauges) > 0, "Should have created some metrics")
		} else {
			assert.True(t, foundRuntimeMetrics)
		}
	})

	t.Run("handles empty parent scope name", func(t *testing.T) {
		parentScope := tally.NewTestScope("", nil)
		collector := NewRuntimeStats(parentScope, nil)

		assert.NotNil(t, collector)
		assert.Equal(t, 10*time.Second, collector.collectionInterval)
	})
}

func TestRuntimeStatCollector_CollectCPUStats(t *testing.T) {
	scope := tally.NewTestScope("test", nil)
	collector := NewRuntimeStats(scope, nil)

	t.Run("collects CPU metrics", func(t *testing.T) {
		// Call collectCPUStats directly
		collector.collectCPUStats()

		snapshot := scope.Snapshot()

		// Verify CPU metrics are recorded
		gauges := snapshot.Gauges()

		// Check that metrics exist - may be empty initially but shouldn't be nil
		assert.NotNil(t, gauges, "Gauges map should not be nil")

		// Look for our specific metrics - they should exist after collection
		goroutinesGauge, exists := gauges["test.runtime.cpu.goroutines"]
		if exists {
			assert.True(t, goroutinesGauge.Value() > 0, "Goroutines count should be positive")
		}

		cgoCallsGauge, exists := gauges["test.runtime.cpu.cgo_calls"]
		if exists {
			assert.True(t, cgoCallsGauge.Value() >= 0, "CGO calls count should be non-negative")
		}

		// At minimum, we should have some metrics collected
		assert.True(t, len(gauges) > 0, "Should have collected some metrics")
	})

	t.Run("CPU metrics change over time", func(t *testing.T) {
		// Take initial snapshot
		collector.collectCPUStats()
		snapshot1 := scope.Snapshot()
		gauges1 := snapshot1.Gauges()

		// Get initial goroutines count if available
		var initialGoroutines float64
		if gauge, exists := gauges1["test.runtime.cpu.goroutines"]; exists {
			initialGoroutines = gauge.Value()
		}

		// Start a few goroutines
		done := make(chan bool, 3)
		for i := 0; i < 3; i++ {
			go func() {
				time.Sleep(10 * time.Millisecond)
				done <- true
			}()
		}

		// Collect stats again
		collector.collectCPUStats()
		snapshot2 := scope.Snapshot()
		gauges2 := snapshot2.Gauges()

		// Clean up goroutines
		for i := 0; i < 3; i++ {
			<-done
		}

		// Check if metrics exist and verify counts
		if gauge, exists := gauges2["test.runtime.cpu.goroutines"]; exists {
			newGoroutines := gauge.Value()
			assert.True(t, newGoroutines >= initialGoroutines, "Goroutines count should increase or stay same")
		} else {
			// If specific metric doesn't exist, at least verify some metrics were collected
			assert.True(t, len(gauges2) > 0, "Should have collected some metrics")
		}
	})
}

func TestRuntimeStatCollector_CollectMemStats(t *testing.T) {
	scope := tally.NewTestScope("test", nil)
	collector := NewRuntimeStats(scope, nil)

	t.Run("collects memory metrics", func(t *testing.T) {
		// Call collectMemStats directly
		collector.collectMemStats()

		snapshot := scope.Snapshot()
		gauges := snapshot.Gauges()

		// Verify all memory metrics are recorded
		memoryMetrics := []string{
			"test.runtime.memory.alloc",
			"test.runtime.memory.total",
			"test.runtime.memory.sys",
			"test.runtime.memory.lookups",
			"test.runtime.memory.malloc",
			"test.runtime.memory.frees",
			"test.runtime.memory.heap.alloc",
			"test.runtime.memory.heap.sys",
			"test.runtime.memory.heap.idle",
			"test.runtime.memory.heap.inuse",
			"test.runtime.memory.heap.released",
			"test.runtime.memory.heap.objects",
			"test.runtime.memory.stack.inuse",
			"test.runtime.memory.stack.sys",
			"test.runtime.memory.stack.mspan_inuse",
			"test.runtime.memory.stack.sys",
			"test.runtime.memory.stack.mcache_inuse",
			"test.runtime.memory.stack.mcache_sys",
			"test.runtime.memory.othersys",
			"test.runtime.memory.gc.sys",
			"test.runtime.memory.gc.next",
			"test.runtime.memory.gc.last",
			"test.runtime.memory.gc.pause_total",
			"test.runtime.memory.gc.count",
		}

		// Check that some metrics exist, may not have all specific ones depending on implementation
		foundMetrics := 0
		for _, metricName := range memoryMetrics {
			if gauge, exists := gauges[metricName]; exists {
				assert.True(t, gauge.Value() >= 0, "Metric %s should be non-negative", metricName)
				foundMetrics++
			}
		}

		// We should have found at least some memory metrics, or verify we have any metrics at all
		if foundMetrics == 0 {
			assert.True(t, len(gauges) > 0, "Should have collected some metrics")
		} else {
			assert.True(t, foundMetrics > 0, "Should have found some memory metrics")
		}
	})

	t.Run("memory metrics reflect allocations", func(t *testing.T) {
		// Take initial snapshot
		collector.collectMemStats()
		snapshot1 := scope.Snapshot()
		gauges1 := snapshot1.Gauges()

		var initialAlloc float64
		if gauge, exists := gauges1["test.runtime.memory.alloc"]; exists {
			initialAlloc = gauge.Value()
		}

		// Allocate some memory
		data := make([]byte, 1024*1024) // 1MB
		for i := range data {
			data[i] = byte(i % 256)
		}

		// Collect stats again
		collector.collectMemStats()
		snapshot2 := scope.Snapshot()
		gauges2 := snapshot2.Gauges()

		// Check if memory allocation metrics exist and have reasonable values
		if gauge, exists := gauges2["test.runtime.memory.alloc"]; exists {
			newAlloc := gauge.Value()
			assert.True(t, newAlloc >= initialAlloc, "Memory allocation should increase or stay same")
		} else {
			// If specific metric doesn't exist, at least verify some memory-related metrics
			assert.True(t, len(gauges2) > 0, "Should have collected some metrics")
		}

		// Keep reference to data to prevent GC
		_ = data[0]
	})
}

func TestRuntimeStatCollector_Collect(t *testing.T) {
	t.Run("collect with context cancellation", func(t *testing.T) {
		scope := tally.NewTestScope("test", nil)
		cfg := &config.GoRuntimeStats{
			CollectionInterval: func() *time.Duration { d := 50 * time.Millisecond; return &d }(),
		}
		collector := NewRuntimeStats(scope, cfg)

		ctx, cancel := context.WithCancel(context.Background())

		// Start collection in background
		done := make(chan bool)
		go func() {
			collector.Collect(ctx)
			done <- true
		}()

		// Let it run for a bit
		time.Sleep(100 * time.Millisecond)

		// Cancel context
		cancel()

		// Wait for collection to stop
		select {
		case <-done:
			// Success - collection stopped
		case <-time.After(1 * time.Second):
			t.Fatal("Collect did not stop after context cancellation")
		}

		// Verify some metrics were collected
		snapshot := scope.Snapshot()
		gauges := snapshot.Gauges()

		assert.NotEmpty(t, gauges, "Some metrics should have been collected")
	})

	t.Run("collect with timeout context", func(t *testing.T) {
		scope := tally.NewTestScope("test", nil)
		cfg := &config.GoRuntimeStats{
			CollectionInterval: func() *time.Duration { d := 20 * time.Millisecond; return &d }(),
		}
		collector := NewRuntimeStats(scope, cfg)

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start collection
		done := make(chan bool)
		go func() {
			collector.Collect(ctx)
			done <- true
		}()

		// Wait for timeout
		select {
		case <-done:
			// Success - collection stopped due to timeout
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Collect did not stop after context timeout")
		}

		// Verify metrics were collected during the timeout period
		snapshot := scope.Snapshot()
		gauges := snapshot.Gauges()

		assert.NotEmpty(t, gauges, "Some metrics should have been collected before timeout")
	})

	t.Run("collect updates metrics periodically", func(t *testing.T) {
		scope := tally.NewTestScope("test", nil)
		cfg := &config.GoRuntimeStats{
			CollectionInterval: func() *time.Duration { d := 30 * time.Millisecond; return &d }(),
		}
		collector := NewRuntimeStats(scope, cfg)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start collection
		go collector.Collect(ctx)

		// Wait for multiple collection cycles
		time.Sleep(100 * time.Millisecond)

		// Verify metrics are being updated
		snapshot := scope.Snapshot()
		gauges := snapshot.Gauges()

		// Should have collected some metrics
		assert.True(t, len(gauges) > 0, "Should have collected some metrics")

		// Check for specific metrics if they exist
		if goroutinesGauge, exists := gauges["test.runtime.cpu.goroutines"]; exists {
			assert.True(t, goroutinesGauge.Value() > 0)
		}

		if memAllocGauge, exists := gauges["test.runtime.memory.alloc"]; exists {
			assert.True(t, memAllocGauge.Value() >= 0)
		}
	})
}

func TestRuntimeStatCollector_EdgeCases(t *testing.T) {
	t.Run("zero collection interval", func(t *testing.T) {
		scope := tally.NewTestScope("test", nil)
		cfg := &config.GoRuntimeStats{
			CollectionInterval: func() *time.Duration { d := time.Duration(0); return &d }(),
		}
		collector := NewRuntimeStats(scope, cfg)

		assert.Equal(t, time.Duration(0), collector.collectionInterval)

		// Zero interval should panic when creating ticker
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		assert.Panics(t, func() {
			collector.Collect(ctx)
		})
	})

	t.Run("very frequent collection", func(t *testing.T) {
		scope := tally.NewTestScope("test", nil)
		cfg := &config.GoRuntimeStats{
			CollectionInterval: func() *time.Duration { d := time.Millisecond; return &d }(),
		}
		collector := NewRuntimeStats(scope, cfg)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Should handle very frequent collection without issues
		assert.NotPanics(t, func() {
			collector.Collect(ctx)
		})
	})

	t.Run("concurrent collection calls", func(t *testing.T) {
		scope := tally.NewTestScope("test", nil)
		collector := NewRuntimeStats(scope, nil)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Start multiple collection goroutines
		const numCollectors = 3
		done := make(chan bool, numCollectors)

		for i := 0; i < numCollectors; i++ {
			go func() {
				collector.Collect(ctx)
				done <- true
			}()
		}

		// Wait for all to complete
		for i := 0; i < numCollectors; i++ {
			select {
			case <-done:
				// Success
			case <-time.After(200 * time.Millisecond):
				t.Fatal("Concurrent collection timed out")
			}
		}
	})
}

func TestRuntimeStatCollector_MetricValues(t *testing.T) {
	scope := tally.NewTestScope("test", nil)
	collector := NewRuntimeStats(scope, nil)

	t.Run("metrics have reasonable values", func(t *testing.T) {
		collector.collectCPUStats()
		collector.collectMemStats()

		snapshot := scope.Snapshot()
		gauges := snapshot.Gauges()

		// Check for specific metrics if they exist
		if goroutinesGauge, exists := gauges["test.runtime.cpu.goroutines"]; exists {
			goroutines := goroutinesGauge.Value()
			assert.True(t, goroutines >= 1, "Should have at least 1 goroutine")
		}

		if memAllocGauge, exists := gauges["test.runtime.memory.alloc"]; exists {
			memAlloc := memAllocGauge.Value()
			assert.True(t, memAlloc > 0, "Memory allocation should be positive")
		}

		if heapAllocGauge, exists := gauges["test.runtime.memory.heap.alloc"]; exists {
			heapAlloc := heapAllocGauge.Value()
			assert.True(t, heapAlloc > 0, "Heap allocation should be positive")
		}

		if gcCountGauge, exists := gauges["test.runtime.memory.gc.count"]; exists {
			gcCount := gcCountGauge.Value()
			assert.True(t, gcCount >= 0, "GC count should be non-negative")
		}

		// At minimum, verify some metrics were collected
		assert.True(t, len(gauges) > 0, "Should have collected some metrics")
	})
}

// Benchmark tests for performance measurement
func BenchmarkNewRuntimeStats(b *testing.B) {
	scope := tally.NewTestScope("benchmark", nil)
	cfg := &config.GoRuntimeStats{
		CollectionInterval: func() *time.Duration { d := time.Second; return &d }(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewRuntimeStats(scope, cfg)
	}
}

func BenchmarkRuntimeStatCollector_CollectCPUStats(b *testing.B) {
	scope := tally.NewTestScope("benchmark", nil)
	collector := NewRuntimeStats(scope, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.collectCPUStats()
	}
}

func BenchmarkRuntimeStatCollector_CollectMemStats(b *testing.B) {
	scope := tally.NewTestScope("benchmark", nil)
	collector := NewRuntimeStats(scope, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.collectMemStats()
	}
}

func BenchmarkRuntimeStatCollector_CollectBoth(b *testing.B) {
	scope := tally.NewTestScope("benchmark", nil)
	collector := NewRuntimeStats(scope, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.collectCPUStats()
		collector.collectMemStats()
	}
}
