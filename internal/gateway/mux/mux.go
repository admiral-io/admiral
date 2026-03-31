package mux

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"go.admiral.io/admiral/internal/config"
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
}

type Route struct {
	Path    string
	Handler http.Handler
}

func New(unaryInterceptors []grpc.UnaryServerInterceptor, assets http.FileSystem, metricsHandler http.Handler, cfg config.Server) (*Mux, error) {

	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(unaryInterceptors...))

	jsonGateway := runtime.NewServeMux(
		//runtime.WithForwardResponseOption(newCustomResponseForwarder(sessionService)),
		runtime.WithErrorHandler(customErrorHandler),
		runtime.WithMarshalerOption(
			runtime.MIMEWildcard,
			&runtime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					UseProtoNames:   false,
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{},
			},
		),
		//runtime.WithIncomingHeaderMatcher(customHeaderMatcher),
	)

	httpMux := http.NewServeMux()
	httpMux.Handle("/", &assetHandler{
		FileSystem: assets,
		FileServer: http.FileServer(assets),
		Next:       jsonGateway,
	})

	if cfg.EnablePprof {
		httpMux.HandleFunc("/debug/pprof/", pprof.Index)
	}

	if metricsHandler != nil {
		httpMux.Handle("/metrics", metricsHandler)
	}

	mux := &Mux{
		JSONGateway: jsonGateway,
		GRPCServer:  grpcServer,
		HTTPMux:     httpMux,
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
