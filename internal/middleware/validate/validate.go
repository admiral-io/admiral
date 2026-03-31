package validate

import (
	"fmt"

	"buf.build/go/protovalidate"
	protovalidatemiddleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/middleware"
)

const Name = "middleware.validate"

func New(_ *config.Config, logger *zap.Logger, scope tally.Scope) (middleware.Middleware, error) {
	validator, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create proto validator: %w", err)
	}

	return &mid{
		logger:    logger.Named("validate"),
		scope:     scope.SubScope("validate"),
		validator: validator,
	}, nil
}

type mid struct {
	logger    *zap.Logger
	scope     tally.Scope
	validator protovalidate.Validator
}

func (m *mid) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return protovalidatemiddleware.UnaryServerInterceptor(m.validator)
}
