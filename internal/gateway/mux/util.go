package mux

import (
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"
)

// isBrowser returns true if the request looks like a browser page navigation
// (as opposed to an API call or SPA fetch()).
func isBrowser(h http.Header) bool {
	if mode := h.Get("Sec-Fetch-Mode"); mode != "" {
		return mode == "navigate"
	}

	if h.Get("X-Requested-With") != "" {
		return false
	}

	return acceptsHTML(h.Get("Accept"))
}

// isBrowserFromMetadata performs the same check as isBrowser but reads from
// gRPC metadata (forwarded by grpc-gateway via customHeaderMatcher).
func isBrowserFromMetadata(md metadata.MD) bool {
	if modes := md.Get("grpcgateway-sec-fetch-mode"); len(modes) > 0 {
		return modes[0] == "navigate"
	}

	if xrw := md.Get("grpcgateway-x-requested-with"); len(xrw) > 0 {
		return false
	}

	if accepts := md.Get("grpcgateway-accept"); len(accepts) > 0 {
		return acceptsHTML(accepts[0])
	}

	return false
}

func acceptsHTML(accept string) bool {
	for _, d := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(strings.SplitN(d, ";", 2)[0])
		if mt == "text/html" {
			return true
		}
	}

	return false
}

func GetCookieValue(headerValues []string, key string) (string, error) {
	if key == "" {
		return "", http.ErrNoCookie
	}

	request := http.Request{Header: http.Header{"Cookie": headerValues}}
	c, err := request.Cookie(key)
	if err != nil {
		return "", err
	}

	return c.Value, nil
}
