package main

// Uncomment after deleting wire.go and wire_gen.go: they also define Initialize.
//
// import (
// 	"github.com/kienvan102/monorepo-microservices-template/core/jsonfile"
// 	"github.com/kienvan102/monorepo-microservices-template/core/logger"
// 	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/business"
// 	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/jsrunner"
// 	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/repository/mongodb"
// 	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/settings"
// )
//
// // One check per wire.Bind in wire.go.
// var (
// 	_ business.ScriptCatalog = (*jsrunner.Catalog)(nil)
// 	_ business.ScriptRunner  = (*jsrunner.Runner)(nil)
// 	_ business.Connector     = (*mongodb.Connector)(nil)
// )
//
// // Initialize builds the mongoanalyzer service for this
// // deployment: each business port is bound to the adapter implementing it.
// func Initialize(cfg settings.Config, log logger.Logger, catalog *jsrunner.Catalog) *business.Analyzer {
// 	connector := mongodb.NewConnector()
// 	runner := jsrunner.NewRunner(catalog)
// 	writer := jsonfile.NewFileWriter()
// 	analyzer := business.NewAnalyzer(cfg, log, catalog, connector, runner, writer)
// 	return analyzer
// }
