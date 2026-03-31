package mux

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

var apiPattern = regexp.MustCompile(`^(/auth/|/api/v\d+/)`)

type assetHandler struct {
	FileSystem http.FileSystem
	FileServer http.Handler
	Next       http.Handler
}

func (a *assetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".ico") ||
		strings.HasSuffix(r.URL.Path, ".svg") ||
		strings.HasSuffix(r.URL.Path, ".webp") {
		if !strings.Contains(r.URL.Path[1:], "/") {
			if f, err := a.FileSystem.Open(r.URL.Path); err == nil {
				defer func() { _ = f.Close() }()
				w.Header().Set("Cache-Control", "public, max-age=86400")
				http.ServeContent(w, r, r.URL.Path, time.Time{}, f)
				return
			}
		}
	}

	if apiPattern.MatchString(r.URL.Path) || r.URL.Path == "/healthcheck" {
		a.Next.ServeHTTP(w, r)
		return
	}

	if f, err := a.FileSystem.Open(r.URL.Path); err != nil {
		r.URL.Path = "/"
	} else {
		_ = f.Close()
	}

	a.FileServer.ServeHTTP(w, r)
}
