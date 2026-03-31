package gateway

import (
	"go.admiral.io/admiral/internal/endpoint"
	"go.admiral.io/admiral/internal/endpoint/healthcheck"
	"go.admiral.io/admiral/internal/middleware"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/database"
)

var Services = service.Factory{
	{Name: database.Name, Factory: database.New},
}

var Middleware = middleware.Factory{}

var Endpoints = endpoint.Factory{
	healthcheck.Name: healthcheck.New,
}

var CoreComponentFactory = &ComponentFactory{
	Services:   Services,
	Middleware: Middleware,
	Endpoints:  Endpoints,
}
