package gateway

import (
	"go.admiral.io/admiral/internal/endpoint"
	authnep "go.admiral.io/admiral/internal/endpoint/authn"
	healthcheckep "go.admiral.io/admiral/internal/endpoint/healthcheck"
	"go.admiral.io/admiral/internal/middleware"
	authnmw "go.admiral.io/admiral/internal/middleware/authn"
	authzmw "go.admiral.io/admiral/internal/middleware/authz"
	statsmw "go.admiral.io/admiral/internal/middleware/stats"
	validatemw "go.admiral.io/admiral/internal/middleware/validate"
	"go.admiral.io/admiral/internal/service"
	authnsvc "go.admiral.io/admiral/internal/service/authn"
	databasesvc "go.admiral.io/admiral/internal/service/database"
	objstoresvc "go.admiral.io/admiral/internal/service/objectstorage"
	sessionsvc "go.admiral.io/admiral/internal/service/session"
)

var Services = service.Factory{
	{Name: databasesvc.Name, Factory: databasesvc.New},
	{Name: sessionsvc.Name, Factory: sessionsvc.New},
	{Name: authnsvc.Name, Factory: authnsvc.New},
	{Name: objstoresvc.Name, Factory: objstoresvc.New},
}

var Middleware = middleware.Factory{
	{Name: statsmw.Name, Factory: statsmw.New},
	{Name: authnmw.Name, Factory: authnmw.New},
	{Name: authzmw.Name, Factory: authzmw.New},
	{Name: validatemw.Name, Factory: validatemw.New},
}

var Endpoints = endpoint.Factory{
	authnep.Name:       authnep.New,
	healthcheckep.Name: healthcheckep.New,
}

var CoreComponentFactory = &ComponentFactory{
	Services:   Services,
	Middleware: Middleware,
	Endpoints:  Endpoints,
}
