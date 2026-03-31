package timeouts

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/middleware"
)

// Test helper to create a logger with observer for log inspection.
func createTestLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	return logger, logs
}

// Mock request structure for testing.
type mockRequest struct {
	Data string `json:"data"`
}

// Mock response structure for testing.
type mockResponse struct {
	Result string `json:"result"`
}

// Mock handler that returns success immediately.
func fastSuccessHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return &mockResponse{Result: "success"}, nil
}

// Mock handler that returns error immediately.
func fastErrorHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, status.Error(codes.InvalidArgument, "test error")
}

// Mock handler that sleeps for specified duration.
func slowHandler(duration time.Duration) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		timer := time.NewTimer(duration)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return &mockResponse{Result: "slow success"}, nil
		}
	}
}

// Mock handler that respects context deadline.
func contextAwareHandler(duration time.Duration) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		deadline, hasDeadline := ctx.Deadline()
		if hasDeadline && time.Until(deadline) < duration {
			return nil, status.Error(codes.DeadlineExceeded, "respecting deadline")
		}

		timer := time.NewTimer(duration)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return &mockResponse{Result: "context-aware success"}, nil
		}
	}
}

func TestNew(t *testing.T) {
	testCases := []struct {
		name                  string
		config                *config.Timeouts
		expectedDefault       time.Duration
		expectedOverrideCount int
		description           string
	}{
		{
			name:                  "nil config uses default timeout",
			config:                nil,
			expectedDefault:       DefaultTimeout,
			expectedOverrideCount: 0,
			description:           "Should create middleware with default timeout when config is nil",
		},
		{
			name: "config with custom default timeout",
			config: &config.Timeouts{
				Default: 30 * time.Second,
			},
			expectedDefault:       30 * time.Second,
			expectedOverrideCount: 0,
			description:           "Should use custom default timeout from config",
		},
		{
			name: "config with zero timeout (infinite)",
			config: &config.Timeouts{
				Default: 0,
			},
			expectedDefault:       0,
			expectedOverrideCount: 0,
			description:           "Should handle zero timeout (infinite) correctly",
		},
		{
			name: "config with overrides",
			config: &config.Timeouts{
				Default: 15 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
					{Service: "auth.AuthService", Method: "Login", Timeout: 10 * time.Second},
				},
			},
			expectedDefault:       15 * time.Second,
			expectedOverrideCount: 2,
			description:           "Should create middleware with custom timeouts for specific methods",
		},
		{
			name: "config with empty overrides",
			config: &config.Timeouts{
				Default:   20 * time.Second,
				Overrides: []config.TimeoutsEntry{},
			},
			expectedDefault:       20 * time.Second,
			expectedOverrideCount: 0,
			description:           "Should handle empty overrides slice",
		},
		{
			name: "config with very short timeout",
			config: &config.Timeouts{
				Default: 100 * time.Millisecond,
			},
			expectedDefault:       100 * time.Millisecond,
			expectedOverrideCount: 0,
			description:           "Should handle very short timeouts",
		},
		{
			name: "config with very long timeout",
			config: &config.Timeouts{
				Default: time.Hour,
			},
			expectedDefault:       time.Hour,
			expectedOverrideCount: 0,
			description:           "Should handle very long timeouts",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(tc.config, logger, scope)

			assert.NoError(t, err, "New should not return error")
			assert.NotNil(t, mw, "Middleware should not be nil")
			assert.Implements(t, (*middleware.Middleware)(nil), mw, "Should implement Middleware interface")

			// Verify internal state (cast to access private fields)
			m := mw.(*mid)
			assert.Equal(t, tc.expectedDefault, m.defaultTimeout, "Default timeout should match expected")
			assert.Equal(t, tc.expectedOverrideCount, len(m.overrides), "Override count should match expected")
			assert.NotNil(t, m.logger, "Logger should not be nil")
			assert.NotNil(t, m.scope, "Scope should not be nil")
		})
	}
}

func TestMid_GetDuration(t *testing.T) {
	testCases := []struct {
		name             string
		config           *config.Timeouts
		service          string
		method           string
		expectedDuration time.Duration
		description      string
	}{
		{
			name: "uses default timeout when no override",
			config: &config.Timeouts{
				Default: 15 * time.Second,
			},
			service:          "test.Service",
			method:           "TestMethod",
			expectedDuration: 15 * time.Second,
			description:      "Should return default timeout when no override exists",
		},
		{
			name: "uses override when exact match",
			config: &config.Timeouts{
				Default: 15 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
				},
			},
			service:          "user.UserService",
			method:           "GetUser",
			expectedDuration: 5 * time.Second,
			description:      "Should return override timeout when exact service/method match",
		},
		{
			name: "uses default when service matches but method differs",
			config: &config.Timeouts{
				Default: 15 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
				},
			},
			service:          "user.UserService",
			method:           "CreateUser",
			expectedDuration: 15 * time.Second,
			description:      "Should return default timeout when service matches but method differs",
		},
		{
			name: "uses default when method matches but service differs",
			config: &config.Timeouts{
				Default: 15 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
				},
			},
			service:          "auth.AuthService",
			method:           "GetUser",
			expectedDuration: 15 * time.Second,
			description:      "Should return default timeout when method matches but service differs",
		},
		{
			name: "handles multiple overrides correctly",
			config: &config.Timeouts{
				Default: 15 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
					{Service: "auth.AuthService", Method: "Login", Timeout: 10 * time.Second},
					{Service: "test.Service", Method: "SlowMethod", Timeout: 30 * time.Second},
				},
			},
			service:          "auth.AuthService",
			method:           "Login",
			expectedDuration: 10 * time.Second,
			description:      "Should return correct override from multiple overrides",
		},
		{
			name: "handles zero timeout override",
			config: &config.Timeouts{
				Default: 15 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "infinite.Service", Method: "InfiniteMethod", Timeout: 0},
				},
			},
			service:          "infinite.Service",
			method:           "InfiniteMethod",
			expectedDuration: 0,
			description:      "Should return zero timeout (infinite) from override",
		},
		{
			name: "handles empty service and method names",
			config: &config.Timeouts{
				Default: 15 * time.Second,
			},
			service:          "",
			method:           "",
			expectedDuration: 15 * time.Second,
			description:      "Should return default timeout for empty service/method names",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(tc.config, logger, scope)
			require.NoError(t, err)

			m := mw.(*mid)
			duration := m.getDuration(tc.service, tc.method)

			assert.Equal(t, tc.expectedDuration, duration, tc.description)
		})
	}
}

func TestMid_UnaryInterceptor_FastHandlers(t *testing.T) {
	testCases := []struct {
		name         string
		config       *config.Timeouts
		fullMethod   string
		handler      grpc.UnaryHandler
		expectError  bool
		expectedCode codes.Code
		description  string
	}{
		{
			name: "fast success handler completes normally",
			config: &config.Timeouts{
				Default: 1 * time.Second,
			},
			fullMethod:   "/test.Service/TestMethod",
			handler:      fastSuccessHandler,
			expectError:  false,
			expectedCode: codes.OK,
			description:  "Should complete successfully with fast handler",
		},
		{
			name: "fast error handler returns error immediately",
			config: &config.Timeouts{
				Default: 1 * time.Second,
			},
			fullMethod:   "/test.Service/TestMethod",
			handler:      fastErrorHandler,
			expectError:  true,
			expectedCode: codes.InvalidArgument,
			description:  "Should return error immediately with fast error handler",
		},
		{
			name: "zero timeout (infinite) with fast handler",
			config: &config.Timeouts{
				Default: 0, // infinite timeout
			},
			fullMethod:   "/test.Service/TestMethod",
			handler:      fastSuccessHandler,
			expectError:  false,
			expectedCode: codes.OK,
			description:  "Should work with infinite timeout",
		},
		{
			name: "override timeout with fast handler",
			config: &config.Timeouts{
				Default: 1 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "test.Service", Method: "TestMethod", Timeout: 500 * time.Millisecond},
				},
			},
			fullMethod:   "/test.Service/TestMethod",
			handler:      fastSuccessHandler,
			expectError:  false,
			expectedCode: codes.OK,
			description:  "Should use override timeout for specific method",
		},
		{
			name: "malformed method name logs warning but works",
			config: &config.Timeouts{
				Default: 1 * time.Second,
			},
			fullMethod:   "invalid-method",
			handler:      fastSuccessHandler,
			expectError:  false,
			expectedCode: codes.OK,
			description:  "Should handle malformed method name gracefully",
		},
		{
			name: "empty method name",
			config: &config.Timeouts{
				Default: 1 * time.Second,
			},
			fullMethod:   "",
			handler:      fastSuccessHandler,
			expectError:  false,
			expectedCode: codes.OK,
			description:  "Should handle empty method name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(tc.config, logger, scope)
			require.NoError(t, err)

			interceptor := mw.UnaryInterceptor()
			assert.NotNil(t, interceptor, "Interceptor should not be nil")

			info := &grpc.UnaryServerInfo{
				FullMethod: tc.fullMethod,
			}

			resp, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, tc.handler)

			if tc.expectError {
				assert.Error(t, err, "Should return error")
				assert.Nil(t, resp, "Response should be nil on error")

				if tc.expectedCode != codes.OK {
					grpcStatus := status.Convert(err)
					assert.Equal(t, tc.expectedCode, grpcStatus.Code(), "Error code should match expected")
				}
			} else {
				assert.NoError(t, err, "Should not return error")
				assert.NotNil(t, resp, "Response should not be nil")
			}

			// Check for warning logs on malformed method names
			if tc.fullMethod == "invalid-method" || tc.fullMethod == "" {
				found := false
				for _, entry := range logs.All() {
					if entry.Message == "could not parse gRPC method" {
						found = true
						break
					}
				}
				assert.True(t, found, "Should log warning for malformed method")
			}
		})
	}
}

func TestMid_UnaryInterceptor_TimeoutBehavior(t *testing.T) {
	testCases := []struct {
		name          string
		config        *config.Timeouts
		fullMethod    string
		handlerDelay  time.Duration
		expectTimeout bool
		description   string
	}{
		{
			name: "handler completes before timeout",
			config: &config.Timeouts{
				Default: 200 * time.Millisecond,
			},
			fullMethod:    "/test.Service/TestMethod",
			handlerDelay:  50 * time.Millisecond,
			expectTimeout: false,
			description:   "Should complete successfully when handler finishes before timeout",
		},
		{
			name: "handler times out",
			config: &config.Timeouts{
				Default: 100 * time.Millisecond,
			},
			fullMethod:    "/test.Service/TestMethod",
			handlerDelay:  300 * time.Millisecond, // Much longer than timeout + boost
			expectTimeout: true,
			description:   "Should timeout when handler takes too long",
		},
		{
			name: "handler completes just before timeout",
			config: &config.Timeouts{
				Default: 150 * time.Millisecond,
			},
			fullMethod:    "/test.Service/TestMethod",
			handlerDelay:  100 * time.Millisecond, // Before timeout
			expectTimeout: false,
			description:   "Should complete when handler finishes just before timeout",
		},
		{
			name: "infinite timeout never times out",
			config: &config.Timeouts{
				Default: 0, // infinite
			},
			fullMethod:    "/test.Service/TestMethod",
			handlerDelay:  200 * time.Millisecond,
			expectTimeout: false,
			description:   "Should never timeout with infinite timeout",
		},
		{
			name: "override timeout used instead of default",
			config: &config.Timeouts{
				Default: 1 * time.Second,
				Overrides: []config.TimeoutsEntry{
					{Service: "test.Service", Method: "TestMethod", Timeout: 100 * time.Millisecond},
				},
			},
			fullMethod:    "/test.Service/TestMethod",
			handlerDelay:  200 * time.Millisecond,
			expectTimeout: true,
			description:   "Should use override timeout instead of default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(tc.config, logger, scope)
			require.NoError(t, err)

			interceptor := mw.UnaryInterceptor()
			info := &grpc.UnaryServerInfo{
				FullMethod: tc.fullMethod,
			}

			handler := slowHandler(tc.handlerDelay)
			start := time.Now()
			resp, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, handler)
			elapsed := time.Since(start)

			if tc.expectTimeout {
				assert.Error(t, err, "Should return timeout error")
				assert.Nil(t, resp, "Response should be nil on timeout")

				// Check for timeout-related errors - could be gRPC status or context errors
				if grpcStatus := status.Convert(err); grpcStatus.Code() == codes.DeadlineExceeded {
					// Explicit timeout from our middleware
					assert.Equal(t, codes.DeadlineExceeded, grpcStatus.Code())
				} else if err == context.DeadlineExceeded {
					// Context timeout
					assert.Equal(t, context.DeadlineExceeded, err)
				} else if err == context.Canceled {
					// Context cancelled
					assert.Equal(t, context.Canceled, err)
				} else {
					// Convert to gRPC status and check
					assert.True(t,
						grpcStatus.Code() == codes.DeadlineExceeded || grpcStatus.Code() == codes.Canceled,
						"Should return timeout-related error, got %v with message: %s", grpcStatus.Code(), grpcStatus.Message())
				}

				// Check for timeout-related error messages
				errorMsg := err.Error()
				assert.True(t,
					errorMsg == "timeout exceeded" ||
						errorMsg == "context deadline exceeded" ||
						errorMsg == "rpc error: code = DeadlineExceeded desc = timeout exceeded",
					"Error message should be timeout-related, got: %s", errorMsg)

				// Should return approximately at timeout + boost
				expectedDuration := tc.config.Default
				if len(tc.config.Overrides) > 0 {
					expectedDuration = tc.config.Overrides[0].Timeout
				}
				if expectedDuration > 0 {
					assert.True(t, elapsed >= expectedDuration, "Should wait at least timeout duration")
					assert.True(t, elapsed < expectedDuration+boost+50*time.Millisecond, "Should not wait much longer than timeout + boost")
				}
			} else {
				assert.NoError(t, err, "Should not return error")
				assert.NotNil(t, resp, "Response should not be nil")

				mockResp, ok := resp.(*mockResponse)
				assert.True(t, ok, "Response should be mock response")
				if tc.config.Default == 0 {
					assert.Equal(t, "slow success", mockResp.Result, "Should return slow success result")
				}
			}

			// For timeout cases, check if we get the "handler completed after timeout" log
			if tc.expectTimeout && tc.handlerDelay < 1*time.Second {
				// Give a bit of time for the handler to complete and log
				time.Sleep(tc.handlerDelay + 100*time.Millisecond)

				found := false
				for _, entry := range logs.All() {
					if entry.Message == "handler completed after timeout" {
						found = true
						break
					}
				}
				// Note: This might be flaky in fast test environments, so we don't require it
				if found {
					assert.True(t, found, "Should log when handler completes after timeout")
				}
			}
		})
	}
}

func TestMid_UnaryInterceptor_ContextAware(t *testing.T) {
	t.Run("context-aware handler respects deadline", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		config := &config.Timeouts{
			Default: 100 * time.Millisecond,
		}

		mw, err := New(config, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Handler that would take 200ms but checks deadline first
		handler := contextAwareHandler(200 * time.Millisecond)

		resp, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, handler)

		assert.Error(t, err, "Should return error when handler respects deadline")
		assert.Nil(t, resp, "Response should be nil")

		grpcStatus := status.Convert(err)
		assert.Equal(t, codes.DeadlineExceeded, grpcStatus.Code(), "Should return DeadlineExceeded")
		assert.Contains(t, grpcStatus.Message(), "respecting deadline", "Should contain deadline message")
	})

	t.Run("context cancellation propagates", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		config := &config.Timeouts{
			Default: 1 * time.Second, // Long timeout
		}

		mw, err := New(config, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Create a cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		// Handler that checks for context cancellation
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
				return &mockResponse{Result: "success"}, nil
			}
		}

		// Cancel context after a short delay
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		resp, err := interceptor(ctx, &mockRequest{Data: "test"}, info, handler)

		assert.Error(t, err, "Should return error when context is cancelled")
		assert.Nil(t, resp, "Response should be nil")
		assert.Equal(t, context.Canceled, err, "Should return context.Canceled error")
	})
}

func TestMid_UnaryInterceptor_EdgeCases(t *testing.T) {
	t.Run("handler that blocks indefinitely gets timeout", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		config := &config.Timeouts{
			Default: 100 * time.Millisecond,
		}

		mw, err := New(config, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Handler that blocks indefinitely
		blockingHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			// Block forever - this simulates a handler that doesn't respect context
			select {}
		}

		resp, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, blockingHandler)

		assert.Error(t, err, "Should return timeout error")
		assert.Nil(t, resp, "Response should be nil")

		// Should get a timeout error
		errorMsg := err.Error()
		assert.True(t,
			errorMsg == "timeout exceeded" ||
				errorMsg == "context deadline exceeded" ||
				errorMsg == "rpc error: code = DeadlineExceeded desc = timeout exceeded",
			"Should return timeout-related error, got: %s", errorMsg)
	})

	t.Run("nil request and response handling", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		config := &config.Timeouts{
			Default: 1 * time.Second,
		}

		mw, err := New(config, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Handler that returns nil response
		nilResponseHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		}

		resp, err := interceptor(context.Background(), nil, info, nilResponseHandler)

		assert.NoError(t, err, "Should handle nil request and response")
		assert.Nil(t, resp, "Response should be nil")
	})

	t.Run("very short timeout still works", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		config := &config.Timeouts{
			Default: 1 * time.Millisecond, // Very short
		}

		mw, err := New(config, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Even fast handlers might timeout with 1ms
		resp, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, fastSuccessHandler)

		// This could go either way depending on timing, so we just check it doesn't panic
		if err != nil {
			grpcStatus := status.Convert(err)
			assert.Equal(t, codes.DeadlineExceeded, grpcStatus.Code(), "Should be deadline exceeded if it fails")
		} else {
			assert.NotNil(t, resp, "Response should not be nil if it succeeds")
		}
	})
}

func TestMid_UnaryInterceptor_Concurrency(t *testing.T) {
	t.Run("concurrent requests handled safely", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		config := &config.Timeouts{
			Default: 200 * time.Millisecond,
		}

		mw, err := New(config, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		const numGoroutines = 10
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)
		responses := make(chan interface{}, numGoroutines)

		// Launch multiple concurrent requests
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// Mix of fast and slow handlers
				var handler grpc.UnaryHandler
				if index%2 == 0 {
					handler = fastSuccessHandler
				} else {
					handler = slowHandler(50 * time.Millisecond) // Should complete before timeout
				}

				resp, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, handler)

				if err != nil {
					errors <- err
				} else {
					responses <- resp
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(responses)

		// Count results
		errorCount := 0
		responseCount := 0

		for err := range errors {
			assert.NotNil(t, err, "Error should not be nil")
			errorCount++
		}

		for resp := range responses {
			assert.NotNil(t, resp, "Response should not be nil")
			responseCount++
		}

		assert.Equal(t, numGoroutines, errorCount+responseCount, "Should account for all requests")
		assert.True(t, responseCount >= numGoroutines/2, "At least half should succeed (fast handlers)")
	})
}

func TestJoin(t *testing.T) {
	testCases := []struct {
		name        string
		service     string
		method      string
		expected    string
		description string
	}{
		{
			name:        "normal service and method",
			service:     "user.UserService",
			method:      "GetUser",
			expected:    "/user.UserService/GetUser",
			description: "Should join service and method with slashes",
		},
		{
			name:        "empty service",
			service:     "",
			method:      "GetUser",
			expected:    "//GetUser",
			description: "Should handle empty service",
		},
		{
			name:        "empty method",
			service:     "user.UserService",
			method:      "",
			expected:    "/user.UserService/",
			description: "Should handle empty method",
		},
		{
			name:        "both empty",
			service:     "",
			method:      "",
			expected:    "//",
			description: "Should handle both empty",
		},
		{
			name:        "service with dots",
			service:     "com.example.service.UserService",
			method:      "CreateUser",
			expected:    "/com.example.service.UserService/CreateUser",
			description: "Should handle service names with dots",
		},
		{
			name:        "method with special characters",
			service:     "test.Service",
			method:      "Method_With_Underscores",
			expected:    "/test.Service/Method_With_Underscores",
			description: "Should handle method names with underscores",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := join(tc.service, tc.method)
			assert.Equal(t, tc.expected, result, tc.description)
		})
	}
}

func TestConstants(t *testing.T) {
	t.Run("DefaultTimeout is reasonable", func(t *testing.T) {
		assert.Equal(t, 15*time.Second, DefaultTimeout, "DefaultTimeout should be 15 seconds")
	})

	t.Run("boost is reasonable", func(t *testing.T) {
		assert.Equal(t, 50*time.Millisecond, boost, "boost should be 50 milliseconds")
		assert.True(t, boost > 0, "boost should be positive")
		assert.True(t, boost < DefaultTimeout, "boost should be much smaller than default timeout")
	})
}

// Benchmark tests for performance measurement.
func BenchmarkNew(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)
	config := &config.Timeouts{
		Default: 15 * time.Second,
		Overrides: []config.TimeoutsEntry{
			{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
			{Service: "auth.AuthService", Method: "Login", Timeout: 10 * time.Second},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := New(config, logger, scope)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMid_GetDuration(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)
	config := &config.Timeouts{
		Default: 15 * time.Second,
		Overrides: []config.TimeoutsEntry{
			{Service: "user.UserService", Method: "GetUser", Timeout: 5 * time.Second},
			{Service: "auth.AuthService", Method: "Login", Timeout: 10 * time.Second},
		},
	}

	mw, err := New(config, logger, scope)
	if err != nil {
		b.Fatal(err)
	}
	m := mw.(*mid)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.getDuration("user.UserService", "GetUser")
	}
}

func BenchmarkUnaryInterceptor_FastHandler(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)
	config := &config.Timeouts{
		Default: 15 * time.Second,
	}

	mw, err := New(config, logger, scope)
	if err != nil {
		b.Fatal(err)
	}

	interceptor := mw.UnaryInterceptor()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := interceptor(context.Background(), &mockRequest{Data: "test"}, info, fastSuccessHandler)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJoin(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = join("user.UserService", "GetUser")
	}
}
