package orchestration

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/uber-go/tally/v4"
	"go.uber.org/zap"

	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/service"
	"go.admiral.io/admiral/internal/service/database"
	"go.admiral.io/admiral/internal/service/objectstorage"
	"go.admiral.io/admiral/internal/store"
)

const Name = "service.orchestration"

// Service encapsulates the job lifecycle state machine: job completion,
// revision status derivation, output capture, dependency promotion,
// run queue advancement, and change set reconciliation.
type Service struct {
	jobStore       *store.JobStore
	revisionStore  *store.RevisionStore
	runStore       *store.RunStore
	appStore       *store.ApplicationStore
	envStore       *store.EnvironmentStore
	variableStore  *store.VariableStore
	componentStore *store.ComponentStore
	moduleStore    *store.ModuleStore
	runnerStore    *store.RunnerStore
	changeSetStore *store.ChangeSetStore
	objStore       objectstorage.Service
	objBucket      string
	baseURL        string
	logger         *zap.Logger
}

// New constructs the orchestration service. It pulls database and object
// storage from the service registry and builds its own stores. Service
// init order in gateway.Services guarantees database and objectstorage
// are registered before this constructor runs.
func New(cfg *config.Config, logger *zap.Logger, _ tally.Scope) (service.Service, error) {
	db, err := service.GetService[database.Service](database.Name)
	if err != nil {
		return nil, err
	}
	gormDB := db.GormDB()

	jobStore, err := store.NewJobStore(gormDB)
	if err != nil {
		return nil, err
	}
	revisionStore, err := store.NewRevisionStore(gormDB)
	if err != nil {
		return nil, err
	}
	runStore, err := store.NewRunStore(gormDB)
	if err != nil {
		return nil, err
	}
	appStore, err := store.NewApplicationStore(gormDB)
	if err != nil {
		return nil, err
	}
	envStore, err := store.NewEnvironmentStore(gormDB)
	if err != nil {
		return nil, err
	}
	variableStore, err := store.NewVariableStore(gormDB)
	if err != nil {
		return nil, err
	}
	componentStore, err := store.NewComponentStore(gormDB)
	if err != nil {
		return nil, err
	}
	moduleStore, err := store.NewModuleStore(gormDB)
	if err != nil {
		return nil, err
	}
	runnerStore, err := store.NewRunnerStore(gormDB)
	if err != nil {
		return nil, err
	}
	changeSetStore, err := store.NewChangeSetStore(gormDB)
	if err != nil {
		return nil, err
	}

	objStore, err := service.GetService[objectstorage.Service](objectstorage.Name)
	if err != nil {
		return nil, fmt.Errorf("object storage is required: %w", err)
	}

	return &Service{
		jobStore:       jobStore,
		revisionStore:  revisionStore,
		runStore:       runStore,
		appStore:       appStore,
		envStore:       envStore,
		variableStore:  variableStore,
		componentStore: componentStore,
		moduleStore:    moduleStore,
		runnerStore:    runnerStore,
		changeSetStore: changeSetStore,
		objStore:       objStore,
		objBucket:      cfg.Services.ObjectStorage.Bucket,
		baseURL:        strings.TrimRight(cfg.Server.ExternalURL, "/"),
		logger:         logger.Named("orchestration"),
	}, nil
}

// BuildBackendConfig generates the Terraform HTTP backend HCL block pointing
// at Admiral's state endpoint.
func (s *Service) BuildBackendConfig(componentID, environmentID uuid.UUID) string {
	if s.baseURL == "" {
		return ""
	}
	stateURL := fmt.Sprintf("%s/api/v1/state/%s/env/%s", s.baseURL, componentID, environmentID)
	lockURL := stateURL + "/lock"
	return fmt.Sprintf(`terraform {
  backend "http" {
    address        = %q
    lock_address   = %q
    unlock_address = %q
    username       = "admiral"
  }
}
`, stateURL, lockURL, lockURL)
}
