package mux

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/session"
)

const (
	xHeader        = "X-"
	xForwardedFor  = "X-Forwarded-For"
	xForwardedHost = "X-Forwarded-Host"
)

type Mux struct {
	JSONGateway *runtime.ServeMux
	GRPCServer  *grpc.Server
	HTTPMux     http.Handler
	RawHTTPMux  *http.ServeMux
}

type Route struct {
	Path    string
	Handler http.Handler
}

func New(unaryInterceptors []grpc.UnaryServerInterceptor, assets http.FileSystem, metricsHandler http.Handler, cfg config.Server) (*Mux, error) {
	sessionService, err := service.GetService[session.Service](session.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get session service: %w", err)
	}

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(unaryInterceptors...))

	jsonGateway := runtime.NewServeMux(
		runtime.WithForwardResponseOption(newCustomResponseForwarder(sessionService)),
		runtime.WithErrorHandler(customErrorHandler),
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard,
			&runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:   true,
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{},
			},
		),
		runtime.WithIncomingHeaderMatcher(customHeaderMatcher),
	)

	httpMux := http.NewServeMux()
	httpMux.Handle("/", &assetHandler{
		FileSystem: assets,
		FileServer: http.FileServer(assets),
		Next:       jsonGateway,
	})

	if cfg.EnablePprof {
		httpMux.HandleFunc("/debug/pprof/", localhostOnly(pprof.Index))
	}

	if metricsHandler != nil {
		httpMux.Handle("/metrics", metricsHandler)
	}

	mux := &Mux{
		JSONGateway: jsonGateway,
		GRPCServer:  grpcServer,
		HTTPMux:     sessionService.HTTPMiddleware()(httpMux),
		RawHTTPMux:  httpMux,
	}
	return mux, nil
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
		m.GRPCServer.ServeHTTP(w, r)
	} else {
		m.HTTPMux.ServeHTTP(w, r)
	}
}

func (m *Mux) EnableGRPCReflection() {
	reflection.Register(m.GRPCServer)
}

func newCustomResponseForwarder(sess session.Service) func(context.Context, http.ResponseWriter, proto.Message) error {
	return func(ctx context.Context, w http.ResponseWriter, resp proto.Message) error {
		md, ok := runtime.ServerMetadataFromContext(ctx)
		if !ok {
			return nil
		}

		if ids := md.HeaderMD.Get("Set-Session-Token"); len(ids) > 0 {
			sess.Put(ctx, "sessionToken", ids[0])

			if !sess.Exists(ctx, "sessionCreatedAt") {
				sess.Put(ctx, "sessionCreatedAt", time.Now().Unix())
			}
		}

		// Redirect when a Location header is set by the handler (auth flow).
		redirects := md.HeaderMD.Get("Location")
		if len(redirects) > 0 {
			code := http.StatusFound
			if st := md.HeaderMD.Get("Location-Status"); len(st) > 0 {
				headerCodeOverride, err := strconv.Atoi(st[0])
				if err != nil {
					return err
				}
				code = headerCodeOverride
			}

			w.Header().Set("Location", redirects[0])
			w.WriteHeader(code)
		}

		return nil
	}
}

func customHeaderMatcher(key string) (string, bool) {
	key = textproto.CanonicalMIMEHeaderKey(key)

	switch key {
	case "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Accept":
		return runtime.MetadataPrefix + key, true
	}

	if strings.HasPrefix(key, xHeader) {
		if key != xForwardedFor && key != xForwardedHost {
			return runtime.MetadataPrefix + key, true
		}
	}

	return runtime.DefaultHeaderMatcher(key)
}

func customErrorHandler(ctx context.Context, mux *runtime.ServeMux, m runtime.Marshaler, w http.ResponseWriter, req *http.Request, err error) {
	if isBrowser(req.Header) { // Redirect if it's the browser (non-XHR).
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unauthenticated {
			redirectPath := fmt.Sprintf("/auth/login?redirect_url=%s", url.QueryEscape(req.RequestURI))
			http.Redirect(w, req, redirectPath, http.StatusFound)
			return
		}

		if strings.HasPrefix(req.URL.Path, "/auth/") {
			errorMessage := "Authentication error occurred"
			if s, ok := status.FromError(err); ok {
				errorMessage = s.Message()
			}
			redirectPath := fmt.Sprintf("/error?message=%s", url.QueryEscape(errorMessage))
			http.Redirect(w, req, redirectPath, http.StatusFound)
			return
		}
	}

	runtime.DefaultHTTPErrorHandler(ctx, mux, m, w, req, err)
}

func InsecureHandler(handler http.Handler) http.Handler {
	return h2c.NewHandler(handler, &http2.Server{})
}

func localhostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if host != "127.0.0.1" && host != "::1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}
