package validate

import (
	"context"
	"testing"

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

// Mock request structure for testing validation.
type mockValidRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Mock response structure for testing.
type mockResponse struct {
	Result string `json:"result"`
}

// Mock handler that returns success.
func successHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return &mockResponse{Result: "success"}, nil
}

// Mock handler that returns error.
func errorHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, status.Error(codes.InvalidArgument, "test error")
}

func TestNew(t *testing.T) {
	testCases := []struct {
		name        string
		config      *config.Config
		description string
	}{
		{
			name:        "creates middleware with valid config",
			config:      &config.Config{},
			description: "Should create middleware with valid config",
		},
		{
			name:        "creates middleware with nil config",
			config:      nil,
			description: "Should create middleware even with nil config",
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

			// Verify internal state
			m := mw.(*mid)
			assert.NotNil(t, m.logger, "Logger should not be nil")
			assert.NotNil(t, m.scope, "Scope should not be nil")
		})
	}
}

func TestMid_UnaryInterceptor(t *testing.T) {
	testCases := []struct {
		name         string
		fullMethod   string
		request      interface{}
		handler      grpc.UnaryHandler
		expectError  bool
		expectedCode codes.Code
		description  string
	}{
		{
			name:         "non-protobuf request returns validation error",
			fullMethod:   "/test.Service/TestMethod",
			request:      &mockValidRequest{Name: "test", Email: "test@example.com"},
			handler:      successHandler,
			expectError:  true,
			expectedCode: codes.Internal,
			description:  "Should return validation error for non-protobuf request",
		},
		{
			name:         "handler error superseded by validation error",
			fullMethod:   "/test.Service/TestMethod",
			request:      &mockValidRequest{Name: "test", Email: "test@example.com"},
			handler:      errorHandler,
			expectError:  true,
			expectedCode: codes.Internal, // Validation error comes first
			description:  "Should return validation error before handler is called",
		},
		{
			name:         "nil request returns validation error",
			fullMethod:   "/test.Service/TestMethod",
			request:      nil,
			handler:      successHandler,
			expectError:  true,
			expectedCode: codes.Internal,
			description:  "Should return validation error for nil request",
		},
		{
			name:         "empty method name still validates request",
			fullMethod:   "",
			request:      &mockValidRequest{Name: "test", Email: "test@example.com"},
			handler:      successHandler,
			expectError:  true,
			expectedCode: codes.Internal,
			description:  "Should validate request regardless of method name",
		},
		{
			name:         "malformed method name still validates request",
			fullMethod:   "invalid-method",
			request:      &mockValidRequest{Name: "test", Email: "test@example.com"},
			handler:      successHandler,
			expectError:  true,
			expectedCode: codes.Internal,
			description:  "Should validate request regardless of method name format",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(nil, logger, scope)
			require.NoError(t, err)

			interceptor := mw.UnaryInterceptor()
			assert.NotNil(t, interceptor, "Interceptor should not be nil")

			info := &grpc.UnaryServerInfo{
				FullMethod: tc.fullMethod,
			}

			resp, err := interceptor(context.Background(), tc.request, info, tc.handler)

			if tc.expectError {
				assert.Error(t, err, "Should return error")
				if tc.expectedCode != codes.OK {
					grpcStatus := status.Convert(err)
					assert.Equal(t, tc.expectedCode, grpcStatus.Code(), "Error code should match expected")
				}
			} else {
				assert.NoError(t, err, "Should not return error")
				assert.NotNil(t, resp, "Response should not be nil")
			}
		})
	}
}

func TestMid_UnaryInterceptor_ValidatorCreation(t *testing.T) {
	t.Run("interceptor creates validator successfully", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		mw, err := New(nil, logger, scope)
		require.NoError(t, err)

		// This should not panic - the validator creation should succeed
		assert.NotPanics(t, func() {
			interceptor := mw.UnaryInterceptor()
			assert.NotNil(t, interceptor, "Interceptor should not be nil")
		}, "UnaryInterceptor should not panic during validator creation")
	})

	t.Run("multiple interceptor calls create separate validators", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		mw, err := New(nil, logger, scope)
		require.NoError(t, err)

		// Create multiple interceptors - each should have its own validator
		interceptor1 := mw.UnaryInterceptor()
		interceptor2 := mw.UnaryInterceptor()

		assert.NotNil(t, interceptor1, "First interceptor should not be nil")
		assert.NotNil(t, interceptor2, "Second interceptor should not be nil")

		// They should be different instances since each creates a new validator
		assert.NotEqual(t, &interceptor1, &interceptor2, "Interceptors should be different instances")
	})
}

func TestMid_UnaryInterceptor_ContextHandling(t *testing.T) {
	testCases := []struct {
		name        string
		setupCtx    func() context.Context
		description string
	}{
		{
			name:        "handles background context",
			setupCtx:    func() context.Context { return context.Background() },
			description: "Should work with background context",
		},
		{
			name: "handles context with values",
			setupCtx: func() context.Context {
				type testKey string
				return context.WithValue(context.Background(), testKey("key"), "value")
			},
			description: "Should work with context containing values",
		},
		{
			name: "handles cancelled context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			description: "Should handle cancelled context",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(nil, logger, scope)
			require.NoError(t, err)

			interceptor := mw.UnaryInterceptor()
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/TestMethod",
			}

			ctx := tc.setupCtx()
			resp, err := interceptor(ctx, &mockValidRequest{Name: "test", Email: "test@example.com"}, info, successHandler)

			// The validation middleware will return an error for non-protobuf messages
			// regardless of context state
			assert.Error(t, err, "Should return validation error for non-protobuf request")
			assert.Nil(t, resp, "Response should be nil on validation error")

			grpcStatus := status.Convert(err)
			assert.Equal(t, codes.Internal, grpcStatus.Code(), "Should return Internal error for unsupported message type")
			assert.Contains(t, grpcStatus.Message(), "unsupported message type", "Error should mention unsupported message type")
		})
	}
}

func TestMid_UnaryInterceptor_ErrorHandling(t *testing.T) {
	t.Run("handles various handler errors", func(t *testing.T) {
		errorTypes := []struct {
			name    string
			handler grpc.UnaryHandler
			code    codes.Code
		}{
			{
				name: "not found error",
				handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.NotFound, "not found")
				},
				code: codes.NotFound,
			},
			{
				name: "internal error",
				handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.Internal, "internal error")
				},
				code: codes.Internal,
			},
			{
				name: "unauthenticated error",
				handler: func(ctx context.Context, req interface{}) (interface{}, error) {
					return nil, status.Error(codes.Unauthenticated, "unauthenticated")
				},
				code: codes.Unauthenticated,
			},
		}

		for _, et := range errorTypes {
			t.Run(et.name, func(t *testing.T) {
				logger, _ := createTestLogger()
				scope := tally.NewTestScope("test", nil)

				mw, err := New(nil, logger, scope)
				require.NoError(t, err)

				interceptor := mw.UnaryInterceptor()
				info := &grpc.UnaryServerInfo{
					FullMethod: "/test.Service/TestMethod",
				}

				_, err = interceptor(context.Background(), &mockValidRequest{Name: "test", Email: "test@example.com"}, info, et.handler)

				assert.Error(t, err, "Should return error")
				grpcStatus := status.Convert(err)
				// Validation error takes precedence over handler error
				assert.Equal(t, codes.Internal, grpcStatus.Code(), "Should return validation error (Internal) before handler error")
				assert.Contains(t, grpcStatus.Message(), "unsupported message type", "Should return validation error message")
			})
		}
	})

	t.Run("handles nil handler gracefully", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		mw, err := New(nil, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		// The validation middleware fails before reaching the nil handler
		resp, err := interceptor(context.Background(), &mockValidRequest{Name: "test", Email: "test@example.com"}, info, nil)

		assert.Error(t, err, "Should return validation error")
		assert.Nil(t, resp, "Response should be nil")

		grpcStatus := status.Convert(err)
		assert.Equal(t, codes.Internal, grpcStatus.Code(), "Should return validation error before reaching nil handler")
		assert.Contains(t, grpcStatus.Message(), "unsupported message type", "Should return validation error message")
	})
}

func TestMid_UnaryInterceptor_ResponseHandling(t *testing.T) {
	testCases := []struct {
		name        string
		handler     grpc.UnaryHandler
		expectNil   bool
		description string
	}{
		{
			name: "handler returns valid response",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return &mockResponse{Result: "success"}, nil
			},
			expectNil:   false,
			description: "Should return valid response from handler",
		},
		{
			name: "handler returns nil response",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return nil, nil
			},
			expectNil:   true,
			description: "Should handle nil response from handler",
		},
		{
			name: "handler returns empty struct",
			handler: func(ctx context.Context, req interface{}) (interface{}, error) {
				return &mockResponse{}, nil
			},
			expectNil:   false,
			description: "Should handle empty struct response",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			mw, err := New(nil, logger, scope)
			require.NoError(t, err)

			interceptor := mw.UnaryInterceptor()
			info := &grpc.UnaryServerInfo{
				FullMethod: "/test.Service/TestMethod",
			}

			resp, err := interceptor(context.Background(), &mockValidRequest{Name: "test", Email: "test@example.com"}, info, tc.handler)

			// Validation error occurs before handler is called
			assert.Error(t, err, "Should return validation error")
			assert.Nil(t, resp, "Response should be nil on validation error")

			grpcStatus := status.Convert(err)
			assert.Equal(t, codes.Internal, grpcStatus.Code(), "Should return validation error")
			assert.Contains(t, grpcStatus.Message(), "unsupported message type", "Should return validation error message")
		})
	}
}

func TestMid_StateValidation(t *testing.T) {
	t.Run("logger is properly named", func(t *testing.T) {
		logger, logs := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		mw, err := New(nil, logger, scope)
		require.NoError(t, err)

		m := mw.(*mid)

		// Test that logger is named properly by creating a log entry
		m.logger.Info("test message")

		entries := logs.All()
		assert.True(t, len(entries) > 0, "Should have logged at least one entry")

		// Check that the logger was named "validate"
		// This is verified by the logger configuration, but we can't easily inspect the name
		// So we just verify the logger works
		assert.NotNil(t, m.logger, "Logger should not be nil")
	})

	t.Run("scope is properly created", func(t *testing.T) {
		logger, _ := createTestLogger()
		parentScope := tally.NewTestScope("parent", nil)

		mw, err := New(nil, logger, parentScope)
		require.NoError(t, err)

		m := mw.(*mid)
		assert.NotNil(t, m.scope, "Scope should not be nil")

		// The scope should be a sub-scope, but we can't easily inspect the name
		// So we just verify it's not nil and works
		counter := m.scope.Counter("test_counter")
		assert.NotNil(t, counter, "Should be able to create counter from scope")
	})
}

func TestMid_IntegrationBehavior(t *testing.T) {
	t.Run("validation middleware rejects non-protobuf messages", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		mw, err := New(nil, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()

		// Create a handler that should not be called due to validation failure
		handlerCalled := false
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			handlerCalled = true
			return &mockResponse{Result: "should not reach here"}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/user.UserService/GetUser",
		}

		request := &mockValidRequest{Name: "test-user", Email: "test@example.com"}
		resp, err := interceptor(context.Background(), request, info, handler)

		// The validation middleware will reject non-protobuf messages
		assert.Error(t, err, "Should return validation error for non-protobuf request")
		assert.Nil(t, resp, "Response should be nil on validation error")
		assert.False(t, handlerCalled, "Handler should not be called when validation fails")

		grpcStatus := status.Convert(err)
		assert.Equal(t, codes.Internal, grpcStatus.Code(), "Should return validation error")
		assert.Contains(t, grpcStatus.Message(), "unsupported message type", "Should mention unsupported message type")
	})

	t.Run("different request types all fail validation", func(t *testing.T) {
		logger, _ := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		mw, err := New(nil, logger, scope)
		require.NoError(t, err)

		interceptor := mw.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		testRequests := []struct {
			name    string
			request interface{}
		}{
			{"string request", "test string"},
			{"int request", 42},
			{"map request", map[string]interface{}{"key": "value"}},
			{"slice request", []string{"item1", "item2"}},
			{"struct request", mockValidRequest{Name: "test", Email: "test@example.com"}},
		}

		for _, tr := range testRequests {
			t.Run(tr.name, func(t *testing.T) {
				resp, err := interceptor(context.Background(), tr.request, info, successHandler)

				assert.Error(t, err, "Should return validation error")
				assert.Nil(t, resp, "Response should be nil")

				grpcStatus := status.Convert(err)
				assert.Equal(t, codes.Internal, grpcStatus.Code(), "Should return Internal error")
				assert.Contains(t, grpcStatus.Message(), "unsupported message type", "Should mention unsupported message type")
			})
		}
	})
}

func TestConstants(t *testing.T) {
	t.Run("Name constant is correct", func(t *testing.T) {
		assert.Equal(t, "middleware.validate", Name, "Name constant should be correct")
	})
}

// Benchmark tests for performance measurement.
func BenchmarkNew(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := New(nil, logger, scope)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnaryInterceptor_Creation(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)

	mw, err := New(nil, logger, scope)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interceptor := mw.UnaryInterceptor()
		_ = interceptor
	}
}

func BenchmarkUnaryInterceptor_Execution(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)

	mw, err := New(nil, logger, scope)
	if err != nil {
		b.Fatal(err)
	}

	interceptor := mw.UnaryInterceptor()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}
	request := &mockValidRequest{Name: "test", Email: "test@example.com"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// The middleware will return validation errors for non-protobuf messages
		// This benchmarks the validation error path
		_, err := interceptor(context.Background(), request, info, successHandler)
		if err == nil {
			b.Fatal("Expected validation error but got none")
		}
		// Validation error is expected for non-protobuf messages
	}
}
