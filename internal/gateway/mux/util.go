package mux

import (
	"net/http"
	"strings"
)

func isBrowser(h http.Header) bool {
	directives := strings.Split(h.Get("Accept"), ",")
	for _, d := range directives {
		mt := strings.SplitN(strings.TrimSpace(d), ";", 1)
		if len(mt) > 0 && mt[0] == "text/html" {
			return true
		}
	}

	return false
}
