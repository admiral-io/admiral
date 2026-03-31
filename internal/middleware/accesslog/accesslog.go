package accesslog

import (
	"context"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/gateway/log"
	"go.admiral.io/admiral/internal/gateway/meta"
	"go.admiral.io/admiral/internal/middleware"
)

const Name = "middleware.accesslog"

type mid struct {
	logger      *zap.Logger
	scope       tally.Scope
	statusCodes []codes.Code
}

func New(config *config.AccessLog, logger *zap.Logger, scope tally.Scope) (middleware.Middleware, error) {
	var statusCodes []codes.Code

	// if no filter is provided default to logging all status codes
	if config != nil {
		for _, filter := range config.StatusCodeFilters {
			statusCodes = append(statusCodes, codes.Code(filter))
		}
	}

	return &mid{
		logger:      logger.Named("accesslog"),
		scope:       scope.SubScope("accesslog"),
		statusCodes: statusCodes,
	}, nil
}

func (m *mid) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		service, method, ok := middleware.SplitFullMethod(info.FullMethod)
		if !ok {
			m.logger.Warn("could not parse gRPC method", zap.String("fullMethod", info.FullMethod))
		}
		resp, err := handler(ctx, req)
		s := status.Convert(err)
		if s == nil {
			s = status.New(codes.OK, "")
		}
		code := s.Code()
		// common logger context fields
		fields := []zap.Field{
			zap.String("service", service),
			zap.String("method", method),
			zap.Int("statusCode", int(code)),
			zap.String("status", code.String()),
		}

		if m.validStatusCode(code) {
			// if err is returned from handler, log error details only
			// as response body will be nil
			if err != nil {
				reqBody, err := meta.APIBody(req)
				if err != nil {
					return nil, err
				}
				fields = append(fields, log.ProtoField("requestBody", reqBody))
				fields = append(fields, zap.String("error", s.Message()))
				m.logger.Error("gRPC", fields...)
			} else {
				m.logger.Info("gRPC", fields...)
			}
		}
		return resp, err
	}
}

func (m *mid) validStatusCode(c codes.Code) bool {
	if len(m.statusCodes) == 0 {
		return true
	}
	for _, code := range m.statusCodes {
		if c == code {
			return true
		}
	}
	return false
}
