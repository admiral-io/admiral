package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"go.admiral.io/admiral/internal/model"
)

// Backend is the per-source-type interface. Each method receives the full
// Source (URL + source_config) and an optional Credential (nil for anonymous
// operations against public sources).
type Backend interface {
	// Probe performs a lightweight authentication check against the source.
	// Returns nil if the credential authenticates successfully (or if the
	// source is reachable anonymously when cred is nil).
	Probe(ctx context.Context, cred *model.Credential, src *model.Source) error

	// Fetch retrieves the source's content at the given ref and returns a
	// local directory. The caller MUST invoke FetchResult.Cleanup when done.
	Fetch(ctx context.Context, cred *model.Credential, src *model.Source, opts FetchOptions) (*FetchResult, error)

	// ListVersions queries the external system for available versions of the
	// source artifact. For GIT these are tags; for TERRAFORM the module
	// version list; for HELM chart versions from index.yaml; for OCI the
	// registry tags. Returns ErrOperationNotSupported when the backend does
	// not implement version discovery (e.g., HTTP archives have no versioning).
	ListVersions(ctx context.Context, cred *model.Credential, src *model.Source) ([]Version, error)
}

// FetchOptions parameterizes a Fetch call. URL + credential come from the
// Source + caller-supplied Credential; Ref/Root are the per-call variables.
type FetchOptions struct {
	Ref  string // branch / tag / commit / chart version / registry version
	Root string // optional subtree to extract (post-fetch)
}

// FetchResult describes successfully materialized source content.
type FetchResult struct {
	Dir      string // filesystem path containing the content
	Revision string // resolved identifier (e.g., commit SHA)
	Digest   string // content-addressing digest (e.g., "sha1:<commit>")
	Cleanup  func() // caller MUST invoke when done
}

// Version describes a single published version of a source's artifact.
type Version struct {
	Name        string
	PublishedAt *time.Time
	Description string
}

// ErrOperationNotSupported is returned by a backend when the caller requests
// an operation the backend doesn't implement yet (typically Fetch or
// ListVersions for source types we haven't built out).
var ErrOperationNotSupported = fmt.Errorf("operation not supported for this source type")

// Factory constructs a Backend. Registered at init time.
type Factory func() Backend

var registry = map[string]Factory{}

// Register associates a Factory with a SourceType
// (e.g., model.SourceTypeGit). Panics on duplicate registration.
func Register(sourceType string, f Factory) {
	if _, exists := registry[sourceType]; exists {
		panic(fmt.Sprintf("backend: duplicate registration for source type %q", sourceType))
	}
	registry[sourceType] = f
}

// For returns the Backend registered for the given SourceType. Returns an
// error if no backend is registered.
func For(sourceType string) (Backend, error) {
	f, ok := registry[sourceType]
	if !ok {
		return nil, fmt.Errorf("no backend registered for source type %q", sourceType)
	}
	return f(), nil
}

// =============================================================================
// Shared security helpers
// =============================================================================

const defaultProbeTimeout = 10 * time.Second

var allowedTargetSchemes = map[string]struct{}{
	"http":  {},
	"https": {},
	"ssh":   {},
	"git":   {},
	"oci":   {},
}

// ValidateTargetURL parses target and rejects disallowed schemes and hosts
// that resolve to non-routable addresses. Applied at the entry of each
// backend operation for early rejection of obviously bad inputs. Does NOT
// defend against DNS rebinding on its own; the hardened HTTP client from
// NewProbeClient does post-resolve enforcement.
//
// Not applicable to SCP-style SSH URLs (`git@host:path`). Callers holding
// such URLs should extract the host themselves and call ValidateHostNotBlocked.
func ValidateTargetURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("target URL has no scheme: %q", raw)
	}
	if _, ok := allowedTargetSchemes[u.Scheme]; !ok {
		return fmt.Errorf("scheme %q is not permitted for target URL", u.Scheme)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("target URL has no host: %q", raw)
	}
	return ValidateHostNotBlocked(u.Hostname())
}

// ValidateHostNotBlocked resolves host and returns an error if any resolved
// IP is in a non-routable class we refuse to probe (loopback, link-local,
// unspecified, multicast). RFC 1918 private ranges are permitted to support
// corporate intranet sources (GitHub Enterprise on 10.x, etc.).
func ValidateHostNotBlocked(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host %q: %w", host, err)
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("host %q resolves to a non-routable address (%s)", host, ip)
		}
	}
	return nil
}

// isBlockedIP returns true for address classes we refuse to connect to.
// Private (RFC 1918) ranges are intentionally NOT blocked.
func isBlockedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast()
}

// NewProbeClient returns an HTTP client suitable for backend probes:
//   - Bounded timeouts.
//   - A custom Dialer inspects the resolved IP post-DNS-lookup and aborts
//     connections to blocked ranges -- defeats DNS rebinding.
//   - Redirects refused (a probe that 3xx's is suspicious).
func NewProbeClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   defaultProbeTimeout,
		KeepAlive: 30 * time.Second,
		ControlContext: func(_ context.Context, _ string, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("probe: split dialed address %q: %w", address, err)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("probe: could not parse dialed address %q", address)
			}
			if isBlockedIP(ip) {
				return fmt.Errorf("probe: refused dial to non-routable address %s", ip)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: defaultProbeTimeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   defaultProbeTimeout,
			ResponseHeaderTimeout: defaultProbeTimeout,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return fmt.Errorf("probe: redirects are not permitted")
		},
	}
}

// mapProbeStatus converts an HTTP status code from a probe into an error
// (or nil for 200). Used by backends that probe via an HTTP GET.
func mapProbeStatus(code int, probeURL string) error {
	switch code {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("authentication failed against %s (status %d)", probeURL, code)
	case http.StatusNotFound:
		return fmt.Errorf("endpoint not found: %s", probeURL)
	default:
		return fmt.Errorf("unexpected status %d probing %s", code, probeURL)
	}
}

// ociRegistryRoot normalizes an OCI Source URL to `scheme://host`.
func ociRegistryRoot(target string) (string, error) {
	normalized := target
	if strings.HasPrefix(normalized, "oci://") {
		normalized = "https://" + strings.TrimPrefix(normalized, "oci://")
	} else if !strings.Contains(normalized, "://") {
		normalized = "https://" + normalized
	}
	u, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("invalid OCI URL %q: %w", target, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("OCI URL %q has no host", target)
	}
	return u.Scheme + "://" + u.Host, nil
}
