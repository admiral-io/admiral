package gateway

import (
	"context"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"go.admiral.io/admiral/internal/endpoint"
)

func newRegistrar(ctx context.Context, s *grpc.Server, c *grpc.ClientConn, m *runtime.ServeMux, h *http.ServeMux) endpoint.Registrar {
	return &registrar{
		ctx: ctx,
		s:   s,
		c:   c,
		m:   m,
		h:   h,
	}
}

type registrar struct {
	ctx context.Context

	s *grpc.Server
	c *grpc.ClientConn
	m *runtime.ServeMux
	h *http.ServeMux
}

func (r *registrar) GRPCServer() *grpc.Server {
	return r.s
}

func (r *registrar) RegisterJSONGateway(f endpoint.GatewayRegisterAPIEndpointFunc) error {
	return f(r.ctx, r.m, r.c)
}

func (r *registrar) HTTPMux() *http.ServeMux {
	return r.h
}
