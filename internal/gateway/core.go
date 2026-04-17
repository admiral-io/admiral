package gateway

import (
	"go.admiral.io/admiral/internal/endpoint"
	applicationep "go.admiral.io/admiral/internal/endpoint/application"
	authnep "go.admiral.io/admiral/internal/endpoint/authn"
	componentep "go.admiral.io/admiral/internal/endpoint/component"
	credentialep "go.admiral.io/admiral/internal/endpoint/credential"
	deploymentep "go.admiral.io/admiral/internal/endpoint/deployment"
	environmentep "go.admiral.io/admiral/internal/endpoint/environment"
	healthcheckep "go.admiral.io/admiral/internal/endpoint/healthcheck"
	moduleep "go.admiral.io/admiral/internal/endpoint/module"
	runnerep "go.admiral.io/admiral/internal/endpoint/runner"
	sourceep "go.admiral.io/admiral/internal/endpoint/source"
	userep "go.admiral.io/admiral/internal/endpoint/user"
	variableep "go.admiral.io/admiral/internal/endpoint/variable"
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
	applicationep.Name:  applicationep.New,
	authnep.Name:        authnep.New,
	componentep.Name:    componentep.New,
	credentialep.Name:   credentialep.New,
	deploymentep.Name:   deploymentep.New,
	environmentep.Name:  environmentep.New,
	healthcheckep.Name:  healthcheckep.New,
	moduleep.Name:       moduleep.New,
	runnerep.Name:       runnerep.New,
	sourceep.Name:       sourceep.New,
	userep.Name:         userep.New,
	variableep.Name:     variableep.New,
}

var CoreComponentFactory = &ComponentFactory{
	Services:   Services,
	Middleware: Middleware,
	Endpoints:  Endpoints,
}
