// db-analyzer is a run-once CLI deployment of the mongoanalyzer service: its
// CLI transport mounted on the shared process framework.
package main

import (
	"github.com/kienvan102/monorepo-microservices-template/core/app"
	"github.com/kienvan102/monorepo-microservices-template/core/processor"
	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/transport/cli"
)

// main runs every mounted service at the same time (app.New). To add a
// service, mount it on its own line with its own prefix, as in the
// commented-out line (mongoanalyzer2 does not exist). Each added service also
// needs, in this deployment:
//   - wire.go: a builder for it (Initialize2), then `go tool wire`.
//   - go.mod: require + replace github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer2.
//   - config.yaml: its settings under the prefix key (m2:); .env: M2_... variables.
//   - Dockerfile + Dockerfile.dockerignore: copy services/mongoanalyzer2.
//
// app.Commands(...) instead runs one mounted service per invocation, chosen
// by name: db-analyzer [-config ...] [-env-file ...] <name> [flags].
func main() {
	processor.Main(app.New(
		app.Mount(cli.NewHandler(Initialize)),
		// app.Mount(cli2.NewHandler(Initialize2), app.WithPrefix("m2")),
	))
	// or simply: func main() { processor.Main(app.New(app.Mount(cli.NewHandler(Initialize)))) }
}

// main.go after switching to manual_wiring.go:
//
// // db-analyzer is a run-once CLI deployment of the mongoanalyzer service: its
// // CLI transport mounted on the shared process framework.
// package main
//
// import (
// 	"github.com/kienvan102/monorepo-microservices-template/core/app"
// 	"github.com/kienvan102/monorepo-microservices-template/core/processor"
// 	"github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer/transport/cli"
// )
//
// // main runs every mounted service at the same time (app.New). To add a
// // service, mount it on its own line with its own prefix, as in the
// // commented-out line (mongoanalyzer2 does not exist). Each added service also
// // needs, in this deployment:
// //   - manual_wiring.go: a builder for it (Initialize2), written by hand.
// //   - go.mod: require + replace github.com/kienvan102/monorepo-microservices-template/services/mongoanalyzer2.
// //   - config.yaml: its settings under the prefix key (m2:); .env: M2_... variables.
// //   - Dockerfile + Dockerfile.dockerignore: copy services/mongoanalyzer2.
// //
// // app.Commands(...) instead runs one mounted service per invocation, chosen
// // by name: db-analyzer [-config ...] [-env-file ...] <name> [flags].
// func main() {
// 	processor.Main(app.New(
// 		app.Mount(cli.NewHandler(Initialize)), // Initialize: manual_wiring.go
// 		// app.Mount(cli2.NewHandler(Initialize2), app.WithPrefix("m2")),
// 	))
// 	// or simply: func main() { processor.Main(app.New(app.Mount(cli.NewHandler(Initialize)))) }
// }
