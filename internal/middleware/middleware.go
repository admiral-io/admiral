package middleware

import (
	"strings"

	"github.com/gobwas/glob"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.admiral.io/admiral/internal/config"
)

type Middleware interface {
	UnaryInterceptor() grpc.UnaryServerInterceptor
}

type Factory map[string]func(*config.Config, *zap.Logger, tally.Scope) (Middleware, error)

func SplitFullMethod(fullMethod string) (service string, method string, ok bool) {
	s := strings.SplitN(fullMethod, "/", 3)
	if len(s) != 3 {
		return "serviceUnknown", "methodUnknown", false
	}
	return s[1], s[2], true
}

func MatchMethodOrResource(pattern, input string) bool {
	if pattern == input || pattern == "*" {
		return true
	}

	g, err := glob.Compile(pattern, '/')
	if err != nil {
		return false
	}
	return g.Match(input)
}
