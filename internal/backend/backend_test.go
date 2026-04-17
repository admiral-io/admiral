package backend

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Security helpers
// =============================================================================

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		name    string
		ip      string
		blocked bool
	}{
		// Blocked
		{"loopback v4", "127.0.0.1", true},
		{"loopback v4 alt", "127.0.0.53", true},
		{"loopback v6", "::1", true},
		{"imds aws/gcp/azure", "169.254.169.254", true},
		{"link-local v4", "169.254.42.1", true},
		{"link-local v6", "fe80::1", true},
		{"unspecified v4", "0.0.0.0", true},
		{"unspecified v6", "::", true},
		{"multicast v4", "224.0.0.1", true},
		{"multicast v6", "ff00::1", true},
		// NOT blocked
		{"cloudflare dns", "1.1.1.1", false},
		{"google dns", "8.8.8.8", false},
		{"rfc1918 10/8", "10.0.0.1", false},
		{"rfc1918 172.16/12", "172.16.5.10", false},
		{"rfc1918 192.168/16", "192.168.1.1", false},
		{"ipv6 ula", "fc00::1", false},
		{"cgn", "100.64.0.1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			require.NotNil(t, ip)
			assert.Equal(t, tc.blocked, isBlockedIP(ip))
		})
	}
}

func TestValidateTargetURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr string // substring
	}{
		// Scheme allowlist
		{"https ok", "https://1.1.1.1/path", ""},
		{"http ok", "http://1.1.1.1/path", ""},
		{"ssh ok", "ssh://git@1.1.1.1/repo", ""},
		{"git ok", "git://1.1.1.1/repo", ""},
		{"oci ok", "oci://1.1.1.1/img", ""},
		{"file rejected", "file:///etc/passwd", "not permitted"},
		{"javascript rejected", "javascript://1.1.1.1/", "not permitted"},
		{"data rejected", "data:text/plain,hello", "not permitted"},
		{"gopher rejected", "gopher://1.1.1.1/", "not permitted"},
		// Missing pieces
		{"no scheme", "just-a-string", "no scheme"},
		{"no host", "https://", "no host"},
		// Blocked hosts (IP literals — no DNS)
		{"imds blocked", "http://169.254.169.254/latest/meta-data/", "non-routable"},
		{"loopback blocked", "http://127.0.0.1:8080/admin", "non-routable"},
		{"unspecified blocked", "http://0.0.0.0/", "non-routable"},
		// RFC 1918 intentionally allowed
		{"rfc1918 allowed", "https://10.0.0.5/github", ""},
		{"rfc1918 192", "https://192.168.1.100/gitlab", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateTargetURL(tc.url)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestValidateHostNotBlocked(t *testing.T) {
	cases := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{"public ip", "1.1.1.1", false},
		{"loopback ip", "127.0.0.1", true},
		{"imds ip", "169.254.169.254", true},
		{"rfc1918 ip", "10.0.0.1", false},
		{"localhost resolves to loopback", "localhost", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHostNotBlocked(tc.host)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOCIRegistryRoot(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"oci scheme", "oci://ghcr.io/acme/charts", "https://ghcr.io", false},
		{"https passthrough", "https://registry.example.com/path", "https://registry.example.com", false},
		{"http passthrough", "http://registry.example.com:5000/path", "http://registry.example.com:5000", false},
		{"bare host defaults to https", "ghcr.io", "https://ghcr.io", false},
		{"bare host with port", "ghcr.io:5000", "https://ghcr.io:5000", false},
		{"empty rejected", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ociRegistryRoot(tc.in)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestMapProbeStatus(t *testing.T) {
	cases := []struct {
		code    int
		substr  string // empty = nil
	}{
		{http.StatusOK, ""},
		{http.StatusUnauthorized, "authentication failed"},
		{http.StatusForbidden, "authentication failed"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "unexpected status 500"},
		{http.StatusBadGateway, "unexpected status 502"},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			err := mapProbeStatus(tc.code, "https://example.com/probe")
			if tc.substr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.substr)
		})
	}
}

// =============================================================================
// Registry
// =============================================================================

func TestRegistry_ForUnknown(t *testing.T) {
	_, err := For("NOT_A_REAL_TYPE_XYZ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backend registered")
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		require.NotNil(t, r, "expected panic on duplicate registration")
		assert.Contains(t, r.(string), "duplicate registration")
	}()
	// GIT is registered at init(); second attempt must panic.
	Register("GIT", func() Backend { return nil })
}

// =============================================================================
// Hardened HTTP client
// =============================================================================

func TestNewProbeClient_RefusesRedirects(t *testing.T) {
	// Testing via httptest would stack the Dialer block (httptest binds to
	// loopback, which the hardened Dialer refuses first). Instead, invoke
	// the CheckRedirect callback directly -- it's the same function the
	// client calls when a redirect appears.
	client := NewProbeClient()
	require.NotNil(t, client.CheckRedirect, "hardened client must set CheckRedirect")

	err := client.CheckRedirect(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirects are not permitted")
}

func TestNewProbeClient_BlocksLoopbackViaDialer(t *testing.T) {
	// httptest.NewServer binds on loopback. The Dialer's ControlContext
	// should refuse the connection post-resolve regardless of what URL
	// pre-resolve validation would say.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewProbeClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)

	_, err = client.Do(req)
	require.Error(t, err)
	assert.True(t, containsInErrChain(err, "refused dial"), "expected refused-dial error, got: %v", err)
}

// =============================================================================
// Archive utilities
// =============================================================================

func TestDetectArchiveFormat(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.com/x.tar.gz", "tar.gz"},
		{"https://example.com/x.tgz", "tar.gz"},
		{"https://example.com/x.tar", "tar"},
		{"https://example.com/x.zip", "zip"},
		{"https://example.com/x.TAR.GZ", "tar.gz"},         // case-insensitive
		{"https://example.com/x.tar.gz?token=abc", "tar.gz"}, // query stripped
		{"https://example.com/x.tgz#frag", "tar.gz"},         // fragment stripped
		{"https://example.com/x.txt", ""},                    // unknown
		{"https://example.com/nothing", ""},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			assert.Equal(t, tc.want, detectArchiveFormat(tc.url))
		})
	}
}

func TestSafeJoin(t *testing.T) {
	cases := []struct {
		name    string
		dest    string
		entry   string
		wantErr bool
	}{
		{"simple", "/tmp/x", "foo/bar", false},
		{"nested", "/tmp/x", "a/b/c", false},
		{"escape with dotdot", "/tmp/x", "../etc/passwd", true},
		{"escape absolute", "/tmp/x", "/etc/passwd", true},
		{"escape mid-path", "/tmp/x", "foo/../../etc", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := safeJoin(tc.dest, tc.entry)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// URL resolution
// =============================================================================

func TestResolveRelativeURL(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		ref     string
		want    string
		wantErr bool
	}{
		{"absolute ref passthrough", "https://host/path/", "https://other/chart.tgz", "https://other/chart.tgz", false},
		{"relative ref resolved", "https://host/charts/index.yaml", "nginx-1.0.0.tgz", "https://host/charts/nginx-1.0.0.tgz", false},
		{"relative ref with ../", "https://host/a/b/index.yaml", "../c/chart.tgz", "https://host/a/c/chart.tgz", false},
		{"absolute path ref", "https://host/a/index.yaml", "/b/chart.tgz", "https://host/b/chart.tgz", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveRelativeURL(tc.base, tc.ref)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

// =============================================================================
// Git URL parsing
// =============================================================================

func TestIsSSHURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"git@github.com:acme/repo.git", true},
		{"ssh://git@github.com/acme/repo.git", true},
		{"https://github.com/acme/repo.git", false},
		{"http://github.com/acme/repo.git", false},
		{"/local/path", false},
		{"git://github.com/acme/repo.git", false},
		{"acme/repo", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			assert.Equal(t, tc.want, isSSHURL(tc.url))
		})
	}
}

func TestIsLikelyCommitSHA(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"abc1234", true},               // 7 hex — minimum
		{"abcdef0123456789abcdef0123456789abcdef01", true}, // 40 hex
		{"ABC1234", true},               // uppercase ok
		{"abcdefg", false},              // non-hex
		{"abc12", false},                // too short (<7)
		{"a" + strings.Repeat("b", 40), false}, // 41 chars, too long
		{"main", false},                 // looks like a branch
		{"v1.2.3", false},               // tag format
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			assert.Equal(t, tc.want, isLikelyCommitSHA(tc.ref))
		})
	}
}

// =============================================================================
// Helpers
// =============================================================================

func containsInErrChain(err error, substr string) bool {
	for err != nil {
		if strings.Contains(err.Error(), substr) {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
