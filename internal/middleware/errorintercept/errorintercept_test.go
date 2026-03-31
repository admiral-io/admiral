package errorintercept

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/middleware"
	healthcheckv1 "go.admiral.io/sdk/proto/admiral/api/healthcheck/v1"
)

// Compile-time check: *Middleware must satisfy middleware.Middleware.
var _ middleware.Middleware = (*Middleware)(nil)

func TestNew(t *testing.T) {
	testCases := []struct {
		name   string
		cfg    *config.Config
		logger *zap.Logger
		scope  tally.Scope
	}{
		{
			name:   "nil parameters",
			cfg:    nil,
			logger: nil,
			scope:  nil,
		},
		{
			name:   "valid parameters",
			cfg:    &config.Config{},
			logger: zap.NewNop(),
			scope:  tally.NoopScope,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := New(tc.cfg, tc.logger, tc.scope)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.IsType(t, &Middleware{}, m)
		})
	}
}

func TestNewMiddleware(t *testing.T) {
	testCases := []struct {
		name   string
		cfg    *config.Config
		logger *zap.Logger
		scope  tally.Scope
	}{
		{
			name:   "nil parameters",
			cfg:    nil,
			logger: nil,
			scope:  nil,
		},
		{
			name:   "valid parameters",
			cfg:    &config.Config{},
			logger: zap.NewNop(),
			scope:  tally.NoopScope,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := NewMiddleware(tc.cfg, tc.logger, tc.scope)
			assert.NoError(t, err)
			assert.NotNil(t, m)
			assert.IsType(t, &Middleware{}, m)
			assert.Empty(t, m.interceptors)
		})
	}
}

func TestMiddleware_AddInterceptor(t *testing.T) {
	t.Run("add single interceptor", func(t *testing.T) {
		m := &Middleware{}
		interceptor := func(err error) error { return err }
		m.AddInterceptor(interceptor)
		assert.Len(t, m.interceptors, 1)
	})

	t.Run("add multiple interceptors", func(t *testing.T) {
		m := &Middleware{}
		interceptor1 := func(err error) error { return err }
		interceptor2 := func(err error) error { return err }
		interceptor3 := func(err error) error { return err }

		m.AddInterceptor(interceptor1)
		m.AddInterceptor(interceptor2)
		m.AddInterceptor(interceptor3)

		assert.Len(t, m.interceptors, 3)
	})

	// BUG: AddInterceptor accepts nil functions without validation.
	// If an error later triggers interceptor execution, calling a nil
	// function will panic with "nil pointer dereference". See the
	// companion test "nil interceptor panics when invoked with error"
	// below that proves this crash.
	t.Run("add nil interceptor stores it", func(t *testing.T) {
		m := &Middleware{}
		var nilInterceptor errorInterceptorFunc

		m.AddInterceptor(nilInterceptor)
		assert.Len(t, m.interceptors, 1)
		assert.Nil(t, m.interceptors[0])
	})
}

func TestMiddleware_UnaryInterceptor_NoError(t *testing.T) {
	testCases := []struct {
		name             string
		interceptorCount int
		expectedResp     interface{}
	}{
		{
			name:             "no interceptors",
			interceptorCount: 0,
			expectedResp:     &healthcheckv1.HealthcheckResponse{},
		},
		{
			name:             "with interceptors",
			interceptorCount: 3,
			expectedResp:     &healthcheckv1.HealthcheckResponse{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Middleware{}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return tc.expectedResp, nil
			}

			for i := 0; i < tc.interceptorCount; i++ {
				m.AddInterceptor(func(err error) error {
					t.Fatal("interceptor should not be called when no error")
					return err
				})
			}

			interceptor := m.UnaryInterceptor()
			resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/method"}, handler)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedResp, resp)
		})
	}
}

func TestMiddleware_UnaryInterceptor_WithError(t *testing.T) {
	t.Run("single interceptor transforms error", func(t *testing.T) {
		m := &Middleware{}
		m.AddInterceptor(func(err error) error {
			return errors.New("transformed error")
		})

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &healthcheckv1.HealthcheckResponse{}, errors.New("original error")
		}

		interceptor := m.UnaryInterceptor()
		resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/method"}, handler)

		require.Error(t, err)
		assert.Equal(t, "transformed error", err.Error())
		assert.Equal(t, &healthcheckv1.HealthcheckResponse{}, resp,
			"response from handler should be returned unchanged even when error is transformed")
	})

	t.Run("multiple interceptors apply in reverse order", func(t *testing.T) {
		m := &Middleware{}
		m.AddInterceptor(func(err error) error {
			return fmt.Errorf("first: %s", err.Error())
		})
		m.AddInterceptor(func(err error) error {
			return fmt.Errorf("second: %s", err.Error())
		})

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &healthcheckv1.HealthcheckResponse{}, errors.New("original")
		}

		interceptor := m.UnaryInterceptor()
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/method"}, handler)

		require.Error(t, err)
		assert.Equal(t, "first: second: original", err.Error())
	})

	t.Run("interceptor returns nil clears error", func(t *testing.T) {
		m := &Middleware{}
		m.AddInterceptor(func(err error) error {
			return nil
		})

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &healthcheckv1.HealthcheckResponse{}, errors.New("original error")
		}

		interceptor := m.UnaryInterceptor()
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/method"}, handler)

		assert.NoError(t, err)
	})

	t.Run("interceptor preserves original error", func(t *testing.T) {
		m := &Middleware{}
		m.AddInterceptor(func(err error) error {
			return err
		})

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &healthcheckv1.HealthcheckResponse{}, errors.New("preserve me")
		}

		interceptor := m.UnaryInterceptor()
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test/method"}, handler)

		require.Error(t, err)
		assert.Equal(t, "preserve me", err.Error())
	})

	t.Run("no interceptors registered passes error through unchanged", func(t *testing.T) {
		m := &Middleware{}
		origErr := errors.New("untouched")

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, origErr
		}

		interceptor := m.UnaryInterceptor()
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)

		require.Error(t, err)
		assert.Equal(t, origErr, err, "with no interceptors the exact error object should pass through")
	})

	t.Run("handler returns both response and error", func(t *testing.T) {
		m := &Middleware{}
		expectedResp := &healthcheckv1.HealthcheckResponse{}

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return expectedResp, errors.New("error")
		}

		interceptor := m.UnaryInterceptor()
		resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)

		assert.Error(t, err)
		assert.Same(t, expectedResp, resp,
			"the exact response pointer from handler should be returned even when there is an error")
	})

	t.Run("handler returns nil response with error", func(t *testing.T) {
		m := &Middleware{}

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, errors.New("error")
		}

		interceptor := m.UnaryInterceptor()
		resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// ---------------------------------------------------------------------------
// Mid-chain nil: an interceptor clears the error, but the loop continues
// and subsequent interceptors receive nil. This is arguably a design issue —
// interceptors may not expect nil input since the loop only starts when
// err != nil. Flagging for discussion.
// ---------------------------------------------------------------------------

func TestMiddleware_UnaryInterceptor_MidChainNilError(t *testing.T) {
	m := &Middleware{}

	var received []string

	// Registered in order: first, second (nils error), third.
	// Executed in reverse: third, second, first.
	m.AddInterceptor(func(err error) error {
		received = append(received, fmt.Sprintf("first got: %v", err))
		if err != nil {
			return errors.New("first result")
		}
		return nil
	})
	m.AddInterceptor(func(err error) error {
		received = append(received, fmt.Sprintf("second got: %v", err))
		return nil // clears the error mid-chain
	})
	m.AddInterceptor(func(err error) error {
		received = append(received, fmt.Sprintf("third got: %v", err))
		return errors.New("third result")
	})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("original")
	}

	interceptor := m.UnaryInterceptor()
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)

	// Execution order: third (gets "original") -> second (gets "third result", returns nil) -> first (gets nil)
	assert.Equal(t, []string{
		"third got: original",
		"second got: third result",
		"first got: <nil>",
	}, received)

	// DESIGN NOTE: The first interceptor receives nil because the second
	// cleared it. The loop does not short-circuit on nil. The first
	// interceptor returns nil (since err is nil in its branch), so the
	// final result is nil — the original error is silently swallowed.
	assert.NoError(t, err,
		"error is nil because mid-chain interceptor cleared it and subsequent interceptors propagated nil")
}

func TestMiddleware_UnaryInterceptor_Context(t *testing.T) {
	testCases := []struct {
		name    string
		ctx     context.Context
		request interface{}
		info    *grpc.UnaryServerInfo
	}{
		{
			name:    "background context",
			ctx:     context.Background(),
			request: &healthcheckv1.HealthcheckRequest{},
			info:    &grpc.UnaryServerInfo{FullMethod: "/healthcheck/v1/check"},
		},
		{
			name: "context with cancel",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return ctx
			}(),
			request: nil,
			info:    &grpc.UnaryServerInfo{FullMethod: "/test/method"},
		},
		{
			name:    "nil request",
			ctx:     context.Background(),
			request: nil,
			info:    &grpc.UnaryServerInfo{FullMethod: "/test/nil"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Middleware{}
			handler := func(ctx context.Context, req interface{}) (interface{}, error) {
				assert.Equal(t, tc.ctx, ctx)
				assert.Equal(t, tc.request, req)
				return &healthcheckv1.HealthcheckResponse{}, nil
			}

			interceptor := m.UnaryInterceptor()
			resp, err := interceptor(tc.ctx, tc.request, tc.info, handler)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

func TestMiddleware_UnaryInterceptor_ErrorChaining(t *testing.T) {
	m := &Middleware{}
	originalErr := errors.New("original error")

	var executionOrder []string

	m.AddInterceptor(func(err error) error {
		executionOrder = append(executionOrder, "first")
		return fmt.Errorf("first: %s", err.Error())
	})

	m.AddInterceptor(func(err error) error {
		executionOrder = append(executionOrder, "second")
		return fmt.Errorf("second: %s", err.Error())
	})

	m.AddInterceptor(func(err error) error {
		executionOrder = append(executionOrder, "third")
		return fmt.Errorf("third: %s", err.Error())
	})

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, originalErr
	}

	interceptor := m.UnaryInterceptor()
	resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)

	assert.Error(t, err)
	assert.Nil(t, resp) // Handler returned nil response
	assert.Equal(t, []string{"third", "second", "first"}, executionOrder)
	assert.Equal(t, "first: second: third: original error", err.Error())
}

func TestMiddleware_UnaryInterceptor_EdgeCases(t *testing.T) {
	t.Run("handler panics", func(t *testing.T) {
		m := &Middleware{}
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			panic("handler panic")
		}

		interceptor := m.UnaryInterceptor()

		assert.Panics(t, func() {
			_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)
		})
	})

	t.Run("interceptor panics", func(t *testing.T) {
		m := &Middleware{}
		m.AddInterceptor(func(err error) error {
			panic("interceptor panic")
		})

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, errors.New("test error")
		}

		interceptor := m.UnaryInterceptor()

		assert.Panics(t, func() {
			_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)
		})
	})

	t.Run("nil handler", func(t *testing.T) {
		m := &Middleware{}
		interceptor := m.UnaryInterceptor()

		assert.Panics(t, func() {
			_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, nil)
		})
	})

	// BUG: A nil interceptor function is accepted by AddInterceptor but
	// will panic with "nil pointer dereference" when invoked during error
	// handling. AddInterceptor should either reject nil or the loop should
	// skip nil entries.
	t.Run("nil interceptor panics when invoked with error", func(t *testing.T) {
		m := &Middleware{}
		var nilFn errorInterceptorFunc
		m.AddInterceptor(nilFn)

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, errors.New("this will trigger the nil interceptor")
		}

		interceptor := m.UnaryInterceptor()

		assert.Panics(t, func() {
			_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)
		}, "calling a nil interceptor function crashes — AddInterceptor should guard against this")
	})

	// A nil interceptor followed by a valid one: the nil interceptor
	// panics before the valid one ever runs.
	t.Run("nil interceptor among valid ones panics", func(t *testing.T) {
		m := &Middleware{}
		m.AddInterceptor(func(err error) error { return err }) // valid (index 0)
		m.AddInterceptor(nil)                                  // nil (index 1) — executed first in reverse
		m.AddInterceptor(func(err error) error { return err }) // valid (index 2) — executed first, fine

		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, errors.New("error")
		}

		interceptor := m.UnaryInterceptor()

		// Reverse order: index 2 (ok) -> index 1 (nil, panic)
		assert.Panics(t, func() {
			_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)
		})
	})
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "middleware.errorintercept", Name)
}

func TestInterceptorInterface(t *testing.T) {
	// NOTE: The Interceptor interface is declared in this package but is
	// not actually used by AddInterceptor (which takes errorInterceptorFunc).
	// This test confirms the interface exists and can be implemented, but
	// there is no integration point — AddInterceptor does not accept an
	// Interceptor. This may be intentional (callers adapt the interface
	// externally) or an incomplete abstraction.
	t.Run("interface implementation", func(t *testing.T) {
		var interceptor Interceptor

		mockImpl := &mockInterceptor{}
		interceptor = mockImpl

		testErr := errors.New("test error")
		result := interceptor.InterceptError(testErr)

		assert.Equal(t, testErr, result)
	})

	t.Run("interface method can be used as interceptor func", func(t *testing.T) {
		mock := &mockInterceptor{}

		m := &Middleware{}
		m.AddInterceptor(mock.InterceptError)
		assert.Len(t, m.interceptors, 1)

		// Verify it actually works end-to-end.
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, errors.New("original")
		}
		interceptor := m.UnaryInterceptor()
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler)

		require.Error(t, err)
		assert.Equal(t, "original", err.Error(), "mockInterceptor passes error through unchanged")
	})
}

type mockInterceptor struct{}

func (m *mockInterceptor) InterceptError(err error) error {
	return err
}

func TestMiddleware_ConcurrentAccess(t *testing.T) {
	m := &Middleware{}

	numGoroutines := 100
	numInterceptors := 10

	for i := 0; i < numInterceptors; i++ {
		interceptorID := i
		m.AddInterceptor(func(err error) error {
			return fmt.Errorf("%s processed by %c", err.Error(), rune('A'+interceptorID))
		})
	}

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("base error")
	}

	interceptor := m.UnaryInterceptor()

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			resp, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/concurrent/test"}, handler)

			assert.Error(t, err)
			assert.Nil(t, resp) // Handler returned nil response
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// ---------------------------------------------------------------------------
// TestUnaryInterceptor_ReusableAcrossCalls verifies that the interceptor
// returned by UnaryInterceptor() can be called multiple times with
// independent results (no shared state leaking between calls).
// ---------------------------------------------------------------------------

func TestUnaryInterceptor_ReusableAcrossCalls(t *testing.T) {
	m := &Middleware{}
	callCount := 0
	m.AddInterceptor(func(err error) error {
		callCount++
		return fmt.Errorf("call %d: %s", callCount, err.Error())
	})

	interceptor := m.UnaryInterceptor()

	// First call with error.
	handler1 := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("err1")
	}
	_, err1 := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler1)
	require.Error(t, err1)
	assert.Equal(t, "call 1: err1", err1.Error())

	// Second call without error — interceptor should not fire.
	handler2 := func(ctx context.Context, req interface{}) (interface{}, error) {
		return "ok", nil
	}
	resp2, err2 := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler2)
	assert.NoError(t, err2)
	assert.Equal(t, "ok", resp2)
	assert.Equal(t, 1, callCount, "interceptor should not have fired for a successful call")

	// Third call with error again.
	handler3 := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("err3")
	}
	_, err3 := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/test"}, handler3)
	require.Error(t, err3)
	assert.Equal(t, "call 2: err3", err3.Error())
}
