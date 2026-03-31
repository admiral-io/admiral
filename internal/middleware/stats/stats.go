package stats

import (
	"context"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/middleware"
)

const Name = "middleware.stats"

func New(_ *config.Config, logger *zap.Logger, scope tally.Scope) (middleware.Middleware, error) {
	return &mid{
		logger: logger.Named("stats"),
		scope:  scope.SubScope("stats"),
	}, nil
}

type mid struct {
	logger *zap.Logger
	scope  tally.Scope
}

func (m *mid) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		service, method, ok := middleware.SplitFullMethod(info.FullMethod)
		if !ok {
			m.logger.Warn("could not parse gRPC method", zap.String("fullMethod", info.FullMethod))
		}

		tags := map[string]string{
			"grpc_service": service,
			"grpc_method":  method,
		}

		t := m.scope.Tagged(tags).Timer("rpc_latency").Start()
		resp, err := handler(ctx, req)
		t.Stop()

		tags["grpc_status"] = status.Convert(err).Code().String()
		m.scope.Tagged(tags).Counter("rpc_total").Inc(1)

		return resp, err
	}
}
