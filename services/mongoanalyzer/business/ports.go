package business

import (
	"context"

	"github.com/kienvan102/monorepo-microservices-template/core/mongoclient"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/entity"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

// ScriptCatalog finds and validates analysis scripts and plans.
type ScriptCatalog interface {
	Discover() ([]string, error)
	Exists(name string) bool
	ValidateTarget(name, collection string) error
	Plan(group, collection string) ([]string, error)
}

// ScriptRunner runs one analysis script against the database in cfg.
type ScriptRunner interface {
	Run(ctx context.Context, name string, cfg settings.MongoConfig) (entity.ScriptResult, error)
}

// Connector opens a MongoDB connection and confirms the configured
// collection exists.
type Connector interface {
	Connect(ctx context.Context, cfg settings.MongoConfig) (mongoclient.Client, error)
}
