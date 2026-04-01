package mux

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBrowser(t *testing.T) {
	testCases := []struct {
		name   string
		expect bool
		header string
	}{
		{name: "simple text/html", expect: true, header: "text/html"},
		{name: "complex browser accept", expect: true, header: "text/html, application/xhtml+xml, application/xml;q=0.9, image/webp, */*;q=0.8"},
		{name: "compact browser accept", expect: true, header: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
		{name: "text/html with quality", expect: false, header: "text/html;q=0.9,application/json"},
		{name: "text/html in middle", expect: true, header: "application/json,text/html,application/xml"},
		{name: "text/html with spaces", expect: true, header: "application/json, text/html , application/xml"},
		{name: "wildcard only", expect: false, header: "*/*"},
		{name: "json and other types", expect: false, header: "application/json, text/plain, */*"},
		{name: "only json", expect: false, header: "application/json"},
		{name: "only xml", expect: false, header: "application/xml"},
		{name: "empty accept", expect: false, header: ""},
		{name: "partial text/html match", expect: false, header: "text/htmlx,application/json"},
		{name: "text/html as substring", expect: false, header: "application/text/html"},
		{name: "case sensitive", expect: false, header: "TEXT/HTML,application/json"},
		{name: "malformed with text/html", expect: false, header: ";;;text/html,,,"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}

			h.Add("Accept", tc.header)
			assert.Equal(t, tc.expect, isBrowser(h))
		})
	}
}

func TestIsBrowser_EdgeCases(t *testing.T) {
	t.Run("no accept header", func(t *testing.T) {
		header := http.Header{}
		result := isBrowser(header)
		assert.False(t, result)
	})

	t.Run("multiple accept header values", func(t *testing.T) {
		header := http.Header{
			"Accept": []string{"application/json", "text/html"},
		}
		// http.Header.Get() only returns the first value
		result := isBrowser(header)
		assert.False(t, result)
	})
}

// based on http.response private member.
type mockResponseWriter struct {
	http.ResponseWriter

	request *http.Request
}

// Mock ResponseWriter that implements the getRequest interface.
type mockResponseWriterWithGetRequest struct {
	http.ResponseWriter
	req *http.Request
}

func (m *mockResponseWriterWithGetRequest) getRequest() *http.Request {
	return m.req
}

func TestRequestHeadersFromResponseWriter(t *testing.T) {
	headers := http.Header{}
	headers.Add("foo", "bar")
	headers.Add("Accept", "text/html")
	m := &mockResponseWriter{request: &http.Request{Header: headers}}

	ret := requestHeadersFromResponseWriter(m)
	assert.EqualValues(t, headers, ret)
}

func TestRequestHeadersFromResponseWriter_GetRequestInterface(t *testing.T) {
	t.Run("ResponseWriter with getRequest method", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("Authorization", "Bearer token")
		headers.Set("Content-Type", "application/json")
		req := &http.Request{Header: headers}

		mock := &mockResponseWriterWithGetRequest{
			req: req,
		}

		result := requestHeadersFromResponseWriter(mock)
		assert.NotNil(t, result)
		assert.Equal(t, "Bearer token", result.Get("Authorization"))
		assert.Equal(t, "application/json", result.Get("Content-Type"))
	})

	t.Run("ResponseWriter with getRequest returning nil", func(t *testing.T) {
		mock := &mockResponseWriterWithGetRequest{
			req: nil,
		}

		result := requestHeadersFromResponseWriter(mock)
		assert.Nil(t, result)
	})
}

func TestRequestHeadersFromResponseWriterWithNilValues(t *testing.T) {
	headers := http.Header{}
	headers["foo"] = nil
	m := &mockResponseWriter{request: &http.Request{Header: headers}}

	ret := requestHeadersFromResponseWriter(m)
	assert.Equal(t, "", ret.Get("foo"))
}

func TestRequestHeadersFromResponseWriterEdgeCases(t *testing.T) {
	t.Run("nil response writer", func(t *testing.T) {
		ret := requestHeadersFromResponseWriter(nil)
		assert.Nil(t, ret)
	})

	t.Run("response writer without request field", func(t *testing.T) {
		type invalidMock struct {
			http.ResponseWriter
		}
		m := &invalidMock{}
		ret := requestHeadersFromResponseWriter(m)
		assert.Nil(t, ret)
	})

	t.Run("response writer with nil request", func(t *testing.T) {
		m := &mockResponseWriter{request: nil}
		ret := requestHeadersFromResponseWriter(m)
		assert.Nil(t, ret)
	})

	t.Run("empty headers", func(t *testing.T) {
		m := &mockResponseWriter{request: &http.Request{Header: http.Header{}}}
		ret := requestHeadersFromResponseWriter(m)
		assert.NotNil(t, ret)
		assert.Empty(t, ret)
	})

	t.Run("multi-value headers", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("Accept", "text/html")
		headers.Add("Accept", "application/json")
		m := &mockResponseWriter{request: &http.Request{Header: headers}}

		ret := requestHeadersFromResponseWriter(m)
		assert.NotNil(t, ret)
		// Function only takes the first value
		assert.Equal(t, "text/html", ret.Get("Accept"))
	})

	t.Run("invalid reflection value", func(t *testing.T) {
		var w http.ResponseWriter
		result := requestHeadersFromResponseWriter(w)
		assert.Nil(t, result)
	})
}

func TestGetCookieValue(t *testing.T) {
	testCases := []struct {
		name        string
		cookies     []string
		key         string
		expected    string
		expectError bool
		errorType   error
	}{
		{
			name:        "first cookie value",
			cookies:     []string{"foo=bar;baz=bang", "baz=bloop"},
			key:         "foo",
			expected:    "bar",
			expectError: false,
		},
		{
			name:        "overridden cookie value",
			cookies:     []string{"foo=bar;baz=bang", "baz=bloop"},
			key:         "baz",
			expected:    "bang",
			expectError: false,
		},
		{
			name:        "missing cookie",
			cookies:     []string{"foo=bar;baz=bang", "baz=bloop"},
			key:         "xyz",
			expected:    "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "empty key",
			cookies:     []string{"foo=bar;baz=bang", "baz=bloop"},
			key:         "",
			expected:    "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "cookie with spaces",
			cookies:     []string{"session = abc123 "},
			key:         "session",
			expected:    " abc123",
			expectError: false,
		},
		{
			name:        "empty header values",
			cookies:     []string{},
			key:         "session",
			expected:    "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "nil header values",
			cookies:     nil,
			key:         "session",
			expected:    "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "empty cookie string",
			cookies:     []string{""},
			key:         "session",
			expected:    "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "cookie with equals in value",
			cookies:     []string{"session=abc=123=def"},
			key:         "session",
			expected:    "abc=123=def",
			expectError: false,
		},
		{
			name:        "cookie with special characters",
			cookies:     []string{"token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			key:         "token",
			expected:    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expectError: false,
		},
		{
			name:        "empty cookie value",
			cookies:     []string{"session=; user=john"},
			key:         "session",
			expected:    "",
			expectError: false,
		},
		{
			name:        "case sensitive cookie name",
			cookies:     []string{"Session=abc123"},
			key:         "session",
			expected:    "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := GetCookieValue(tc.cookies, tc.key)
			if tc.expectError {
				assert.Error(t, err)
				if tc.errorType != nil {
					assert.Equal(t, tc.errorType, err)
				}
				assert.Equal(t, tc.expected, res)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, res)
			}
		})
	}
}

func TestGetCookieValue_AdditionalEdgeCases(t *testing.T) {
	t.Run("duplicate cookie names", func(t *testing.T) {
		// According to HTTP spec, duplicate cookie names should use the first occurrence
		cookies := []string{"session=first; session=second"}
		result, err := GetCookieValue(cookies, "session")
		assert.NoError(t, err)
		assert.Equal(t, "first", result)
	})

	t.Run("cookie with path and domain attributes", func(t *testing.T) {
		cookies := []string{"session=abc123; Path=/; Domain=example.com; HttpOnly"}
		result, err := GetCookieValue(cookies, "session")
		assert.NoError(t, err)
		assert.Equal(t, "abc123", result)
	})

	t.Run("multiple header values with target cookie", func(t *testing.T) {
		cookies := []string{"session=abc123", "user=john; theme=dark"}
		result, err := GetCookieValue(cookies, "theme")
		assert.NoError(t, err)
		assert.Equal(t, "dark", result)
	})
}

// Benchmark tests for performance-critical functions.
func BenchmarkIsBrowser(b *testing.B) {
	header := http.Header{
		"Accept": []string{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isBrowser(header)
	}
}

func BenchmarkGetCookieValue(b *testing.B) {
	cookies := []string{"session=abc123; user=john; theme=dark; token=xyz789"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = GetCookieValue(cookies, "theme")
	}
}
