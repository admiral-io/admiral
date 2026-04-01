package mux

import (
	"net/http"
	"reflect"
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

func requestHeadersFromResponseWriter(w http.ResponseWriter) http.Header {
	rv := reflect.ValueOf(w)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil
	}

	req := rv.FieldByName("request")
	if !req.IsValid() {
		if rw, ok := w.(interface{ getRequest() *http.Request }); ok {
			reqVal := reflect.ValueOf(rw.getRequest())
			if reqVal.IsValid() {
				req = reqVal
			}
		}
	}
	if !req.IsValid() || req.IsNil() {
		return nil
	}

	h := req.Elem().FieldByName("Header")
	if !h.IsValid() {
		return nil
	}

	ret := make(http.Header, h.Len())
	iter := h.MapRange()
	for iter.Next() {
		k := iter.Key().String()
		var v string
		if iter.Value().Len() > 0 {
			v = iter.Value().Index(0).String()
		}
		ret[k] = []string{v}
	}
	return ret
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
