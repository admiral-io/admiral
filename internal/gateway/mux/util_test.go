package mux

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/metadata"
)

func TestIsBrowser(t *testing.T) {
	t.Run("Sec-Fetch-Mode: navigate is browser", func(t *testing.T) {
		h := http.Header{"Sec-Fetch-Mode": []string{"navigate"}}
		assert.True(t, isBrowser(h))
	})

	t.Run("Sec-Fetch-Mode: cors is not browser", func(t *testing.T) {
		h := http.Header{"Sec-Fetch-Mode": []string{"cors"}}
		assert.False(t, isBrowser(h))
	})

	t.Run("Sec-Fetch-Mode: same-origin is not browser", func(t *testing.T) {
		h := http.Header{"Sec-Fetch-Mode": []string{"same-origin"}}
		assert.False(t, isBrowser(h))
	})

	t.Run("Sec-Fetch-Mode: no-cors is not browser", func(t *testing.T) {
		h := http.Header{"Sec-Fetch-Mode": []string{"no-cors"}}
		assert.False(t, isBrowser(h))
	})

	t.Run("X-Requested-With present falls back to non-browser", func(t *testing.T) {
		h := http.Header{
			"Accept":           []string{"text/html"},
			"X-Requested-With": []string{"XMLHttpRequest"},
		}
		assert.False(t, isBrowser(h))
	})

	t.Run("Accept text/html without Sec-Fetch-Mode is browser", func(t *testing.T) {
		h := http.Header{"Accept": []string{"text/html"}}
		assert.True(t, isBrowser(h))
	})

	t.Run("complex browser accept", func(t *testing.T) {
		h := http.Header{"Accept": []string{"text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8"}}
		assert.True(t, isBrowser(h))
	})

	t.Run("JSON only is not browser", func(t *testing.T) {
		h := http.Header{"Accept": []string{"application/json"}}
		assert.False(t, isBrowser(h))
	})

	t.Run("empty headers is not browser", func(t *testing.T) {
		assert.False(t, isBrowser(http.Header{}))
	})

	t.Run("Sec-Fetch-Mode takes priority over Accept", func(t *testing.T) {
		h := http.Header{
			"Sec-Fetch-Mode": []string{"cors"},
			"Accept":         []string{"text/html"},
		}
		assert.False(t, isBrowser(h))
	})
}

func TestIsBrowserFromMetadata(t *testing.T) {
	t.Run("navigate is browser", func(t *testing.T) {
		md := metadata.MD{"grpcgateway-sec-fetch-mode": []string{"navigate"}}
		assert.True(t, isBrowserFromMetadata(md))
	})

	t.Run("cors is not browser", func(t *testing.T) {
		md := metadata.MD{"grpcgateway-sec-fetch-mode": []string{"cors"}}
		assert.False(t, isBrowserFromMetadata(md))
	})

	t.Run("X-Requested-With blocks browser detection", func(t *testing.T) {
		md := metadata.MD{
			"grpcgateway-accept":           []string{"text/html"},
			"grpcgateway-x-requested-with": []string{"XMLHttpRequest"},
		}
		assert.False(t, isBrowserFromMetadata(md))
	})

	t.Run("Accept text/html fallback", func(t *testing.T) {
		md := metadata.MD{"grpcgateway-accept": []string{"text/html,application/json"}}
		assert.True(t, isBrowserFromMetadata(md))
	})

	t.Run("empty metadata is not browser", func(t *testing.T) {
		assert.False(t, isBrowserFromMetadata(metadata.MD{}))
	})
}

func TestAcceptsHTML(t *testing.T) {
	testCases := []struct {
		name   string
		accept string
		expect bool
	}{
		{"simple text/html", "text/html", true},
		{"complex browser accept", "text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8", true},
		{"compact browser accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", true},
		{"text/html with quality", "text/html;q=0.9,application/json", true},
		{"text/html in middle", "application/json,text/html,application/xml", true},
		{"text/html with spaces", "application/json, text/html , application/xml", true},
		{"wildcard only", "*/*", false},
		{"json and other types", "application/json, text/plain, */*", false},
		{"only json", "application/json", false},
		{"only xml", "application/xml", false},
		{"empty accept", "", false},
		{"partial text/html match", "text/htmlx,application/json", false},
		{"text/html as substring", "application/text/html", false},
		{"case sensitive", "TEXT/HTML,application/json", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, acceptsHTML(tc.accept))
		})
	}
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
			name:     "first cookie value",
			cookies:  []string{"foo=bar;baz=bang", "baz=bloop"},
			key:      "foo",
			expected: "bar",
		},
		{
			name:     "overridden cookie value",
			cookies:  []string{"foo=bar;baz=bang", "baz=bloop"},
			key:      "baz",
			expected: "bang",
		},
		{
			name:        "missing cookie",
			cookies:     []string{"foo=bar;baz=bang", "baz=bloop"},
			key:         "xyz",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "empty key",
			cookies:     []string{"foo=bar;baz=bang", "baz=bloop"},
			key:         "",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:     "cookie with spaces",
			cookies:  []string{"session = abc123 "},
			key:      "session",
			expected: " abc123",
		},
		{
			name:        "empty header values",
			cookies:     []string{},
			key:         "session",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:        "nil header values",
			cookies:     nil,
			key:         "session",
			expectError: true,
			errorType:   http.ErrNoCookie,
		},
		{
			name:     "cookie with equals in value",
			cookies:  []string{"session=abc=123=def"},
			key:      "session",
			expected: "abc=123=def",
		},
		{
			name:     "cookie with special characters",
			cookies:  []string{"token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			key:      "token",
			expected: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		},
		{
			name:     "empty cookie value",
			cookies:  []string{"session=; user=john"},
			key:      "session",
			expected: "",
		},
		{
			name:        "case sensitive cookie name",
			cookies:     []string{"Session=abc123"},
			key:         "session",
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
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, res)
			}
		})
	}
}

func BenchmarkIsBrowser(b *testing.B) {
	header := http.Header{
		"Sec-Fetch-Mode": []string{"navigate"},
		"Accept":         []string{"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"},
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
