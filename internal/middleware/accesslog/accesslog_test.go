package accesslog

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

// Mock request structure for testing.
type mockRequest struct {
	Data string `json:"data"`
}

// Mock response structure for testing.
type mockResponse struct {
	Result string `json:"result"`
}

// Mock handler that returns success.
func successHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return &mockResponse{Result: "success"}, nil
}

// Mock handler that returns an error.
func errorHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return nil, status.Error(codes.InvalidArgument, "test error")
}

// Mock handler that returns a specific error code.
func customErrorHandler(code codes.Code, message string) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, status.Error(code, message)
	}
}

func TestNew(t *testing.T) {
	testCases := []struct {
		name                string
		config              *config.AccessLog
		expectedStatusCodes []codes.Code
		description         string
	}{
		{
			name:                "nil config uses default (log all)",
			config:              nil,
			expectedStatusCodes: nil,
			description:         "Should create middleware with nil status code filter (logs all)",
		},
		{
			name: "config with status code filters",
			config: &config.AccessLog{
				StatusCodeFilters: []uint32{uint32(codes.OK), uint32(codes.InvalidArgument)},
			},
			expectedStatusCodes: []codes.Code{codes.OK, codes.InvalidArgument},
			description:         "Should create middleware with specified status code filters",
		},
		{
			name: "config with empty status code filters",
			config: &config.AccessLog{
				StatusCodeFilters: []uint32{},
			},
			expectedStatusCodes: nil,
			description:         "Should create middleware with nil status code filter",
		},
		{
			name: "config with single status code filter",
			config: &config.AccessLog{
				StatusCodeFilters: []uint32{uint32(codes.NotFound)},
			},
			expectedStatusCodes: []codes.Code{codes.NotFound},
			description:         "Should create middleware with single status code filter",
		},
		{
			name: "config with multiple status code filters",
			config: &config.AccessLog{
				StatusCodeFilters: []uint32{
					uint32(codes.OK),
					uint32(codes.InvalidArgument),
					uint32(codes.NotFound),
					uint32(codes.Internal),
				},
			},
			expectedStatusCodes: []codes.Code{codes.OK, codes.InvalidArgument, codes.NotFound, codes.Internal},
			description:         "Should create middleware with multiple status code filters",
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
			if tc.expectedStatusCodes == nil {
				assert.Nil(t, m.statusCodes, "Status codes should be nil")
			} else {
				assert.Equal(t, tc.expectedStatusCodes, m.statusCodes, "Status codes should match expected")
			}
			assert.NotNil(t, m.logger, "Logger should not be nil")
			assert.NotNil(t, m.scope, "Scope should not be nil")
		})
	}
}

func TestMid_UnaryInterceptor(t *testing.T) {
	testCases := []struct {
		name             string
		config           *config.AccessLog
		fullMethod       string
		handler          grpc.UnaryHandler
		request          interface{}
		expectedService  string
		expectedMethod   string
		expectedCode     codes.Code
		expectedLogLevel zapcore.Level
		shouldLog        bool
		expectError      bool
		description      string
	}{
		{
			name:             "successful request logs info",
			config:           nil, // logs all
			fullMethod:       "/test.Service/TestMethod",
			handler:          successHandler,
			request:          &mockRequest{Data: "test"},
			expectedService:  "test.Service",
			expectedMethod:   "TestMethod",
			expectedCode:     codes.OK,
			expectedLogLevel: zapcore.InfoLevel,
			shouldLog:        true,
			expectError:      false,
			description:      "Should log successful request at info level",
		},
		{
			name:             "error request logs error",
			config:           nil, // logs all
			fullMethod:       "/auth.AuthService/Login",
			handler:          errorHandler,
			request:          &mockRequest{Data: "test"},
			expectedService:  "auth.AuthService",
			expectedMethod:   "Login",
			expectedCode:     codes.InvalidArgument,
			expectedLogLevel: zapcore.ErrorLevel,
			shouldLog:        true,
			expectError:      true,
			description:      "Should log error request at error level",
		},
		{
			name: "filtered status code not logged",
			config: &config.AccessLog{
				StatusCodeFilters: []uint32{uint32(codes.NotFound)}, // only log NotFound
			},
			fullMethod:       "/test.Service/TestMethod",
			handler:          successHandler, // returns OK
			request:          &mockRequest{Data: "test"},
			expectedService:  "test.Service",
			expectedMethod:   "TestMethod",
			expectedCode:     codes.OK,
			expectedLogLevel: zapcore.InfoLevel,
			shouldLog:        false, // OK is not in filter
			expectError:      false,
			description:      "Should not log when status code not in filter",
		},
		{
			name: "filtered status code is logged",
			config: &config.AccessLog{
				StatusCodeFilters: []uint32{uint32(codes.InvalidArgument)},
			},
			fullMethod:       "/test.Service/TestMethod",
			handler:          errorHandler, // returns InvalidArgument
			request:          &mockRequest{Data: "test"},
			expectedService:  "test.Service",
			expectedMethod:   "TestMethod",
			expectedCode:     codes.InvalidArgument,
			expectedLogLevel: zapcore.ErrorLevel,
			shouldLog:        true, // InvalidArgument is in filter
			expectError:      true,
			description:      "Should log when status code is in filter",
		},
		{
			name:             "malformed method logs warning",
			config:           nil,
			fullMethod:       "invalid-method",
			handler:          successHandler,
			request:          &mockRequest{Data: "test"},
			expectedService:  "serviceUnknown",
			expectedMethod:   "methodUnknown",
			expectedCode:     codes.OK,
			expectedLogLevel: zapcore.InfoLevel,
			shouldLog:        true,
			expectError:      false,
			description:      "Should handle malformed method and log warning",
		},
		{
			name:             "empty method",
			config:           nil,
			fullMethod:       "",
			handler:          successHandler,
			request:          &mockRequest{Data: "test"},
			expectedService:  "serviceUnknown",
			expectedMethod:   "methodUnknown",
			expectedCode:     codes.OK,
			expectedLogLevel: zapcore.InfoLevel,
			shouldLog:        true,
			expectError:      false,
			description:      "Should handle empty method",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			middleware, err := New(tc.config, logger, scope)
			require.NoError(t, err)

			interceptor := middleware.UnaryInterceptor()
			assert.NotNil(t, interceptor, "Interceptor should not be nil")

			// Create mock server info
			info := &grpc.UnaryServerInfo{
				FullMethod: tc.fullMethod,
			}

			// Execute interceptor
			resp, err := interceptor(context.Background(), tc.request, info, tc.handler)

			// Verify error expectations
			if tc.expectError {
				assert.Error(t, err, "Should return error")
				assert.Nil(t, resp, "Response should be nil on error")
			} else {
				assert.NoError(t, err, "Should not return error")
				assert.NotNil(t, resp, "Response should not be nil")
			}

			// Verify logging behavior
			if tc.shouldLog {
				assert.True(t, logs.Len() > 0, "Should have logged at least one message")

				// Find the main log entry (not the warning about malformed method)
				var mainLogEntry *observer.LoggedEntry
				for _, entry := range logs.All() {
					if entry.Message == "gRPC" {
						mainLogEntry = &entry
						break
					}
				}

				if mainLogEntry != nil {
					assert.Equal(t, tc.expectedLogLevel, mainLogEntry.Level, "Log level should match expected")
					assert.Equal(t, "gRPC", mainLogEntry.Message, "Log message should be 'gRPC'")

					// Verify log fields
					fields := mainLogEntry.Context
					serviceField := findField(fields, "service")
					methodField := findField(fields, "method")
					statusCodeField := findField(fields, "statusCode")
					statusField := findField(fields, "status")

					assert.NotNil(t, serviceField, "Service field should be present")
					assert.NotNil(t, methodField, "Method field should be present")
					assert.NotNil(t, statusCodeField, "StatusCode field should be present")
					assert.NotNil(t, statusField, "Status field should be present")

					if serviceField != nil {
						assert.Equal(t, tc.expectedService, serviceField.String, "Service should match expected")
					}
					if methodField != nil {
						assert.Equal(t, tc.expectedMethod, methodField.String, "Method should match expected")
					}
					if statusCodeField != nil {
						assert.Equal(t, int64(tc.expectedCode), statusCodeField.Integer, "Status code should match expected")
					}
					if statusField != nil {
						assert.Equal(t, tc.expectedCode.String(), statusField.String, "Status string should match expected")
					}

					// Check for error-specific fields
					if tc.expectError {
						errorField := findField(fields, "error")
						assert.NotNil(t, errorField, "Error field should be present for error logs")
					}
				}
			} else {
				// Should not log main gRPC entry, but might log warning for malformed method
				gRPCLogs := 0
				for _, entry := range logs.All() {
					if entry.Message == "gRPC" {
						gRPCLogs++
					}
				}
				assert.Equal(t, 0, gRPCLogs, "Should not have logged main gRPC entry")
			}
		})
	}
}

func TestMid_ValidStatusCode(t *testing.T) {
	testCases := []struct {
		name        string
		statusCodes []codes.Code
		testCode    codes.Code
		expected    bool
		description string
	}{
		{
			name:        "nil filter logs all codes",
			statusCodes: nil,
			testCode:    codes.OK,
			expected:    true,
			description: "Should return true for any code when filter is nil",
		},
		{
			name:        "nil filter logs error codes",
			statusCodes: nil,
			testCode:    codes.Internal,
			expected:    true,
			description: "Should return true for error codes when filter is nil",
		},
		{
			name:        "code in filter",
			statusCodes: []codes.Code{codes.OK, codes.InvalidArgument, codes.NotFound},
			testCode:    codes.InvalidArgument,
			expected:    true,
			description: "Should return true when code is in filter",
		},
		{
			name:        "code not in filter",
			statusCodes: []codes.Code{codes.OK, codes.InvalidArgument},
			testCode:    codes.NotFound,
			expected:    false,
			description: "Should return false when code is not in filter",
		},
		{
			name:        "single code filter match",
			statusCodes: []codes.Code{codes.Internal},
			testCode:    codes.Internal,
			expected:    true,
			description: "Should return true for single code filter match",
		},
		{
			name:        "single code filter no match",
			statusCodes: []codes.Code{codes.Internal},
			testCode:    codes.OK,
			expected:    false,
			description: "Should return false for single code filter no match",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, _ := createTestLogger()
			scope := tally.NewTestScope("test", nil)

			m := &mid{
				logger:      logger,
				scope:       scope,
				statusCodes: tc.statusCodes,
			}

			result := m.validStatusCode(tc.testCode)
			assert.Equal(t, tc.expected, result, tc.description)
		})
	}
}

func TestMid_ErrorHandling(t *testing.T) {
	t.Run("handles nil status gracefully", func(t *testing.T) {
		logger, logs := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		middleware, err := New(nil, logger, scope)
		require.NoError(t, err)

		interceptor := middleware.UnaryInterceptor()

		// Handler that returns nil error (which converts to OK status)
		nilErrorHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &mockResponse{Result: "success"}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/test.Service/TestMethod",
		}

		resp, err := interceptor(context.Background(), &mockRequest{}, info, nilErrorHandler)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, logs.Len() > 0, "Should have logged")

		// Find the main log entry
		for _, entry := range logs.All() {
			if entry.Message == "gRPC" {
				statusCodeField := findField(entry.Context, "statusCode")
				assert.Equal(t, int64(codes.OK), statusCodeField.Integer, "Should default to OK status")
				break
			}
		}
	})

	t.Run("handles various error types", func(t *testing.T) {
		errorTypes := []struct {
			name    string
			handler grpc.UnaryHandler
			code    codes.Code
		}{
			{
				name:    "standard grpc status error",
				handler: customErrorHandler(codes.Unauthenticated, "not authenticated"),
				code:    codes.Unauthenticated,
			},
			{
				name:    "internal server error",
				handler: customErrorHandler(codes.Internal, "internal error"),
				code:    codes.Internal,
			},
			{
				name:    "not found error",
				handler: customErrorHandler(codes.NotFound, "resource not found"),
				code:    codes.NotFound,
			},
		}

		for _, et := range errorTypes {
			t.Run(et.name, func(t *testing.T) {
				logger, logs := createTestLogger()
				scope := tally.NewTestScope("test", nil)

				middleware, err := New(nil, logger, scope)
				require.NoError(t, err)

				interceptor := middleware.UnaryInterceptor()
				info := &grpc.UnaryServerInfo{
					FullMethod: "/test.Service/TestMethod",
				}

				_, err = interceptor(context.Background(), &mockRequest{}, info, et.handler)

				assert.Error(t, err, "Should return error")

				// Verify correct status code is logged
				for _, entry := range logs.All() {
					if entry.Message == "gRPC" {
						statusCodeField := findField(entry.Context, "statusCode")
						assert.Equal(t, int64(et.code), statusCodeField.Integer, "Should log correct status code")
						break
					}
				}
			})
		}
	})
}

func TestMid_LogFields(t *testing.T) {
	t.Run("logs all required fields for success", func(t *testing.T) {
		logger, logs := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		middleware, err := New(nil, logger, scope)
		require.NoError(t, err)

		interceptor := middleware.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/user.UserService/GetUser",
		}

		_, err = interceptor(context.Background(), &mockRequest{Data: "test"}, info, successHandler)
		assert.NoError(t, err)

		// Find the main log entry
		for _, entry := range logs.All() {
			if entry.Message == "gRPC" {
				fields := entry.Context

				// Verify all required fields are present
				requiredFields := []string{"service", "method", "statusCode", "status"}
				for _, fieldName := range requiredFields {
					field := findField(fields, fieldName)
					assert.NotNil(t, field, "Field %s should be present", fieldName)
				}

				// Verify specific values
				serviceField := findField(fields, "service")
				methodField := findField(fields, "method")
				assert.Equal(t, "user.UserService", serviceField.String)
				assert.Equal(t, "GetUser", methodField.String)
				break
			}
		}
	})

	t.Run("logs error-specific fields for failures", func(t *testing.T) {
		logger, logs := createTestLogger()
		scope := tally.NewTestScope("test", nil)

		middleware, err := New(nil, logger, scope)
		require.NoError(t, err)

		interceptor := middleware.UnaryInterceptor()
		info := &grpc.UnaryServerInfo{
			FullMethod: "/user.UserService/GetUser",
		}

		_, err = interceptor(context.Background(), &mockRequest{Data: "test"}, info, errorHandler)
		assert.Error(t, err)

		// Find the main log entry
		for _, entry := range logs.All() {
			if entry.Message == "gRPC" {
				fields := entry.Context

				// Verify error-specific fields
				errorField := findField(fields, "error")
				requestBodyField := findField(fields, "requestBody")

				assert.NotNil(t, errorField, "Error field should be present")
				assert.NotNil(t, requestBodyField, "Request body field should be present")
				assert.Equal(t, "test error", errorField.String)
				break
			}
		}
	})
}

// Helper function to find a field by key in zap context.
func findField(fields []zapcore.Field, key string) *zapcore.Field {
	for _, field := range fields {
		if field.Key == key {
			return &field
		}
	}
	return nil
}

// Benchmark tests for performance measurement.
func BenchmarkNew(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)
	config := &config.AccessLog{
		StatusCodeFilters: []uint32{uint32(codes.OK), uint32(codes.Internal)},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := New(config, logger, scope)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnaryInterceptor_Success(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)

	middleware, err := New(nil, logger, scope)
	if err != nil {
		b.Fatal(err)
	}

	interceptor := middleware.UnaryInterceptor()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := interceptor(context.Background(), &mockRequest{}, info, successHandler)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnaryInterceptor_Error(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)

	middleware, err := New(nil, logger, scope)
	if err != nil {
		b.Fatal(err)
	}

	interceptor := middleware.UnaryInterceptor()
	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/TestMethod",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = interceptor(context.Background(), &mockRequest{}, info, errorHandler)
	}
}

func BenchmarkValidStatusCode(b *testing.B) {
	logger, _ := createTestLogger()
	scope := tally.NewTestScope("benchmark", nil)

	m := &mid{
		logger:      logger,
		scope:       scope,
		statusCodes: []codes.Code{codes.OK, codes.InvalidArgument, codes.NotFound, codes.Internal},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.validStatusCode(codes.InvalidArgument)
	}
}
