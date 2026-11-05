package main

import (
	"github.com/kienvan102/monorepo-microservices-template/commonlib/processor"
	"github.com/kienvan102/monorepo-microservices-template/core/app"
	"github.com/kienvan102/monorepo-microservices-template/services/crawlermonitor/entrypoint/scheduler"
)
func main() {
	processor.Main(app.New(
		app.Mount(scheduler.NewHandler(Initialize)),
	))
}