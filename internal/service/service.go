package service

import (
	"fmt"

	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
)

type Service interface{}

type OrderedFactoryEntry struct {
	Name    string
	Factory func(*config.Config, *zap.Logger, tally.Scope) (Service, error)
}

type Factory []OrderedFactoryEntry

var Registry = map[string]Service{}

func GetService[T any](name string) (T, error) {
	svc, ok := Registry[name]
	if !ok {
		var zero T
		return zero, fmt.Errorf("service %q not found", name)
	}
	typed, ok := svc.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("service %q is not of type %T", name, zero)
	}
	return typed, nil
}
