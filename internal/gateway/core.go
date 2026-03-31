package gateway

import (
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/endpoint/healthcheck"
	"go.admiral.io/admiral/internal/middleware"
	"go.admiral.io/admiral/internal/middleware/authn"
	"go.admiral.io/admiral/internal/middleware/authz"
	"go.admiral.io/admiral/internal/middleware/stats"
	"go.admiral.io/admiral/internal/middleware/validate"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/database"
)

var Services = service.Factory{
	{Name: database.Name, Factory: database.New},
}

var Middleware = middleware.Factory{
	{Name: stats.Name, Factory: stats.New},
	{Name: authn.Name, Factory: authn.New},
	{Name: authz.Name, Factory: authz.New},
	{Name: validate.Name, Factory: validate.New},
}

var Endpoints = endpoint.Factory{
	healthcheck.Name: healthcheck.New,
}

var CoreComponentFactory = &ComponentFactory{
	Services:   Services,
	Middleware: Middleware,
	Endpoints:  Endpoints,
}
