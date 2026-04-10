package stats

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
)

func TestNew(t *testing.T) {
	testCases := []struct {
		name          string
		config        *config.Config
		logger        *zap.Logger
		expectedError bool
	}{
		{
			name:          "successful creation",
			config:        &config.Config{},
			logger:        zaptest.NewLogger(t),
			expectedError: false,
		},
		{
			name:          "nil config",
			config:        nil,
			logger:        zaptest.NewLogger(t),
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scope := tally.NoopScope

			middleware, err := New(tc.config, tc.logger, scope)

			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, middleware)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, middleware)

				// Verify the middleware has the correct type
				statsMiddleware, ok := middleware.(*mid)
				assert.True(t, ok)
				assert.NotNil(t, statsMiddleware.logger)
				assert.NotNil(t, statsMiddleware.scope)
			}
		})
	}
}

func TestMid_UnaryInterceptor(t *testing.T) {
	testCases := []struct {
		name           string
		fullMethod     string
		handlerError   error
		handlerResp    interface{}
		expectedStatus codes.Code
	}{
		{
			name:           "successful request with valid method",
			fullMethod:     "/admiral.v1.UserService/GetUser",
			handlerError:   nil,
			handlerResp:    "success response",
			expectedStatus: codes.OK,
		},
		{
			name:           "request with gRPC error",
			fullMethod:     "/admiral.v1.UserService/CreateUser",
			handlerError:   status.Error(codes.InvalidArgument, "invalid input"),
			handlerResp:    nil,
			expectedStatus: codes.InvalidArgument,
		},
		{
			name:           "request with non-gRPC error",
			fullMethod:     "/admiral.v1.ApplicationService/Deploy",
			handlerError:   errors.New("database connection failed"),
			handlerResp:    nil,
			expectedStatus: codes.Unknown,
		},
		{
			name:           "invalid method format",
			fullMethod:     "invalid-method-format",
			handlerError:   nil,
			handlerResp:    "response",
			expectedStatus: codes.OK,
		},
		{
			name:           "method with only one slash",
			fullMethod:     "/SingleSlash",
			handlerError:   nil,
			handlerResp:    "response",
			expectedStatus: codes.OK,
		},
		{
			name:           "empty method",
			fullMethod:     "",
			handlerError:   nil,
			handlerResp:    "response",
			expectedStatus: codes.OK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create middleware with noop scope for testing
			logger := zaptest.NewLogger(t)
			scope := tally.NoopScope
			middleware := &mid{
				logger: logger,
				scope:  scope,
			}

			// Track if handler was called
			handlerCalled := false

			// Create mock handler
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				handlerCalled = true
				return tc.handlerResp, tc.handlerError
			}

			// Create gRPC server info
			info := &grpc.UnaryServerInfo{
				FullMethod: tc.fullMethod,
			}

			// Execute interceptor
			interceptor := middleware.UnaryInterceptor()
			resp, err := interceptor(context.Background(), "test request", info, handler)

			// Verify results
			assert.True(t, handlerCalled, "handler should have been called")
			assert.Equal(t, tc.handlerResp, resp)
			assert.Equal(t, tc.handlerError, err)

			// Verify error status code matches expected
			if tc.handlerError != nil {
				st := status.Convert(err)
				assert.Equal(t, tc.expectedStatus, st.Code())
			}
		})
	}
}

func TestMid_UnaryInterceptor_ContextPropagation(t *testing.T) {
	t.Run("context is properly passed to handler", func(t *testing.T) {
		// Create middleware
		logger := zaptest.NewLogger(t)
		scope := tally.NoopScope
		middleware := &mid{
			logger: logger,
			scope:  scope,
		}

		// Create context with value
		type testKey string
		ctx := context.WithValue(context.Background(), testKey("test"), "test-value")

		// Create handler that checks context
		var receivedCtx context.Context
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			receivedCtx = ctx
			return "response", nil
		}

		// Create gRPC server info
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Execute interceptor
		interceptor := middleware.UnaryInterceptor()
		_, err := interceptor(ctx, "test request", info, handler)

		// Verify context was passed correctly
		require.NoError(t, err)
		assert.Equal(t, "test-value", receivedCtx.Value(testKey("test")))
	})
}

func TestMid_UnaryInterceptor_MethodParsing(t *testing.T) {
	testCases := []struct {
		name               string
		fullMethod         string
		expectedLogWarning bool
	}{
		{
			name:               "valid full method",
			fullMethod:         "/admiral.v1.UserService/GetUser",
			expectedLogWarning: false,
		},
		{
			name:               "invalid method format - no slashes",
			fullMethod:         "no.slashes.here",
			expectedLogWarning: true,
		},
		{
			name:               "invalid method format - one slash",
			fullMethod:         "/one.slash",
			expectedLogWarning: true,
		},
		{
			name:               "empty method",
			fullMethod:         "",
			expectedLogWarning: true,
		},
		{
			name:               "method with extra slashes",
			fullMethod:         "/service/method/extra",
			expectedLogWarning: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create logger that captures logs
			observedZapCore, observedLogs := observer.New(zapcore.WarnLevel)
			logger := zap.New(observedZapCore)

			// Create middleware
			scope := tally.NoopScope
			middleware := &mid{
				logger: logger.Named("stats"),
				scope:  scope,
			}

			// Create simple handler
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return "response", nil
			}

			// Create gRPC server info
			info := &grpc.UnaryServerInfo{
				FullMethod: tc.fullMethod,
			}

			// Execute interceptor
			interceptor := middleware.UnaryInterceptor()
			_, err := interceptor(context.Background(), "test request", info, handler)

			// Verify no error from interceptor
			assert.NoError(t, err)

			// Check if warning was logged as expected
			logs := observedLogs.All()
			if tc.expectedLogWarning {
				assert.Len(t, logs, 1, "expected warning log")
				if len(logs) > 0 {
					assert.Equal(t, zap.WarnLevel, logs[0].Level)
					assert.Contains(t, logs[0].Message, "could not parse gRPC method")
				}
			} else {
				assert.Len(t, logs, 0, "expected no warning logs")
			}
		})
	}
}

func TestMid_UnaryInterceptor_TimingAndMetrics(t *testing.T) {
	t.Run("interceptor measures timing", func(t *testing.T) {
		// Create middleware
		logger := zaptest.NewLogger(t)
		scope := tally.NoopScope
		middleware := &mid{
			logger: logger,
			scope:  scope,
		}

		// Create handler that takes some time
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			time.Sleep(1 * time.Millisecond) // Small delay to test timing
			return "response", nil
		}

		// Create gRPC server info
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// Measure execution time
		start := time.Now()
		interceptor := middleware.UnaryInterceptor()
		_, err := interceptor(context.Background(), "test request", info, handler)
		elapsed := time.Since(start)

		// Verify the interceptor completed successfully
		assert.NoError(t, err)

		// Verify it took at least the sleep duration (timing overhead is expected)
		assert.GreaterOrEqual(t, elapsed, 1*time.Millisecond)
	})
}

func TestConstants(t *testing.T) {
	t.Run("name constant", func(t *testing.T) {
		assert.Equal(t, "middleware.stats", Name)
	})
}

func TestMidStruct(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		scope := tally.NoopScope

		middleware := &mid{
			logger: logger,
			scope:  scope,
		}

		assert.Equal(t, logger, middleware.logger)
		assert.Equal(t, scope, middleware.scope)
	})
}

func TestMid_UnaryInterceptor_ErrorConversion(t *testing.T) {
	testCases := []struct {
		name         string
		inputError   error
		expectedCode codes.Code
	}{
		{
			name:         "nil error",
			inputError:   nil,
			expectedCode: codes.OK,
		},
		{
			name:         "grpc status error",
			inputError:   status.Error(codes.NotFound, "not found"),
			expectedCode: codes.NotFound,
		},
		{
			name:         "standard error",
			inputError:   errors.New("standard error"),
			expectedCode: codes.Unknown,
		},
		{
			name:         "grpc internal error",
			inputError:   status.Error(codes.Internal, "internal error"),
			expectedCode: codes.Internal,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create middleware
			logger := zaptest.NewLogger(t)
			scope := tally.NoopScope
			middleware := &mid{
				logger: logger,
				scope:  scope,
			}

			// Create handler that returns the test error
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, tc.inputError
			}

			// Create gRPC server info
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/TestMethod",
			}

			// Execute interceptor
			interceptor := middleware.UnaryInterceptor()
			_, err := interceptor(context.Background(), "test request", info, handler)

			// Verify error matches expected
			assert.Equal(t, tc.inputError, err)

			// Verify status code conversion
			st := status.Convert(err)
			assert.Equal(t, tc.expectedCode, st.Code())
		})
	}
}
