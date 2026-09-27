//go:build wireinject

package main

import (
	"github.com/google/wire"

	"github.com/kienvan102/monorepo-microservices-template/core/jsonfile"
	"github.com/kienvan102/monorepo-microservices-template/core/logger"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/business"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/jsrunner"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/repository/mongodb"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
)

// Initialize builds the mongoanalyzer service for this
// deployment: each business port is bound to the adapter implementing it.
// Regenerate wire_gen.go with `go tool wire` in this directory.
func Initialize(cfg settings.Config, log logger.Logger, catalog *jsrunner.Catalog) *business.Analyzer {
	wire.Build(
		wire.Bind(new(business.ScriptCatalog), new(*jsrunner.Catalog)),
		jsrunner.NewRunner,
		wire.Bind(new(business.ScriptRunner), new(*jsrunner.Runner)),
		mongodb.NewConnector,
		wire.Bind(new(business.Connector), new(*mongodb.Connector)),
		jsonfile.NewFileWriter,
		business.NewAnalyzer,
	)
	return nil
}
